package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/Luzifer/rconfig"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

var (
	cfg = struct {
		CheckDefinitionsFile string `flag:"check-definitions-file,c" default:"/etc/elb-instance-status.yml" description:"File or URL containing checks to perform for instance health"`
		UnhealthyThreshold   int64  `flag:"unhealthy-threshold" default:"5" description:"How often does a check have to fail to mark the machine unhealthy"`
		Listen               string `flag:"listen" default:":3000" description:"IP/Port to listen on for ELB health checks"`
		VersionAndExit       bool   `flag:"version" default:"false" description:"Print version and exit"`
	}{}

	version = "dev"

	checks               map[string]checkCommand
	checkResults         = map[string]*checkResult{}
	checkResultsLock     sync.RWMutex
	lastResultRegistered time.Time
)

type checkCommand struct {
	Name     string `yaml:"name"`
	Command  string `yaml:"command"`
	WarnOnly bool   `yaml:"warn-only"`
}

type checkResult struct {
	Check     checkCommand
	IsSuccess bool
	Streak    int64
}

func init() {
	rconfig.Parse(&cfg)

	if cfg.VersionAndExit {
		fmt.Printf("elb-instance-status %s\n", version)
		os.Exit(0)
	}
}

func loadChecks() error {
	var (
		rawChecks []byte
		err       error
	)

	if _, err := os.Stat(cfg.CheckDefinitionsFile); err == nil {
		// We got a local file, read it
		rawChecks, err = ioutil.ReadFile(cfg.CheckDefinitionsFile)
		if err != nil {
			return err
		}
	} else {
		// Check whether we got an URL
		if _, err := url.Parse(cfg.CheckDefinitionsFile); err != nil {
			return errors.New("Definitions file is neither a local file nor a URL")
		}

		// We got an URL, fetch and read it
		resp, err := http.Get(cfg.CheckDefinitionsFile)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		rawChecks, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	}

	tmpResult := map[string]checkCommand{}
	err = yaml.Unmarshal(rawChecks, &tmpResult)

	if err == nil {
		checks = tmpResult
	}

	return err
}

func main() {
	if err := loadChecks(); err != nil {
		log.Fatalf("Unable to read definitions file: %s", err)
	}

	c := cron.New()
	c.AddFunc("@every 1m", spawnChecks)
	c.AddFunc("@every 10m", func() {
		if err := loadChecks(); err != nil {
			log.Printf("Unable to refresh checks: %s", err)
		}
	})
	c.Start()

	spawnChecks()

	r := mux.NewRouter()
	r.HandleFunc("/status", handleELBHealthCheck)
	r.Handle("/metrics", prometheus.Handler())
	http.ListenAndServe(cfg.Listen, r)
}

func spawnChecks() {
	ctx, _ := context.WithTimeout(context.Background(), 59*time.Second)

	for id := range checks {
		go executeAndRegisterCheck(ctx, id)
	}
}

func executeAndRegisterCheck(ctx context.Context, checkID string) {
	check := checks[checkID]
	start := time.Now()

	cmd := exec.Command("/bin/bash", "-c", check.Command)
	err := cmd.Start()

	if err == nil {
		cmdDone := make(chan error)
		go func(cmdDone chan error, cmd *exec.Cmd) { cmdDone <- cmd.Wait() }(cmdDone, cmd)
		loop := true
		for loop {
			select {
			case err = <-cmdDone:
				loop = false
			case <-ctx.Done():
				log.Printf("Execution of check '%s' was killed through context timeout.", checkID)
				cmd.Process.Kill()
				time.Sleep(time.Millisecond)
			}
		}
	}

	success := err == nil

	checkResultsLock.Lock()

	if _, ok := checkResults[checkID]; !ok {
		checkResults[checkID] = &checkResult{
			Check: check,
		}
	}

	if success == checkResults[checkID].IsSuccess {
		checkResults[checkID].Streak++
	} else {
		checkResults[checkID].IsSuccess = success
		checkResults[checkID].Streak = 1
	}

	lastResultRegistered = time.Now()

	if success {
		checkPassing.WithLabelValues(checkID).Set(1)
	} else {
		checkPassing.WithLabelValues(checkID).Set(0)
	}
	checkExecutionTime.WithLabelValues(checkID).Observe(float64(time.Since(start).Nanoseconds()) / float64(time.Microsecond))

	checkResultsLock.Unlock()
}

func handleELBHealthCheck(res http.ResponseWriter, r *http.Request) {
	healthy := true
	start := time.Now()
	buf := bytes.NewBuffer([]byte{})

	checkResultsLock.RLock()
	for _, cr := range checkResults {
		state := ""
		switch {
		case cr.IsSuccess:
			state = "PASS"
		case !cr.IsSuccess && cr.Check.WarnOnly:
			state = "WARN"
		case !cr.IsSuccess && !cr.Check.WarnOnly && cr.Streak < cfg.UnhealthyThreshold:
			state = "CRIT"
		case !cr.IsSuccess && !cr.Check.WarnOnly && cr.Streak >= cfg.UnhealthyThreshold:
			state = "CRIT"
			healthy = false
		}
		fmt.Fprintf(buf, "[%s] %s\n", state, cr.Check.Name)
	}
	checkResultsLock.RUnlock()

	res.Header().Set("X-Collection-Parsed-In", strconv.FormatInt(time.Since(start).Nanoseconds()/int64(time.Microsecond), 10)+"ms")
	res.Header().Set("X-Last-Result-Registered-At", lastResultRegistered.Format(time.RFC1123))
	if healthy {
		currentStatusCode.Set(http.StatusOK)
		res.WriteHeader(http.StatusOK)
	} else {
		currentStatusCode.Set(http.StatusInternalServerError)
		res.WriteHeader(http.StatusInternalServerError)
	}

	io.Copy(res, buf)
}
