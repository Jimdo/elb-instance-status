package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/Luzifer/rconfig"
	"github.com/gorilla/mux"
	"github.com/robfig/cron"
)

var (
	cfg = struct {
		CheckDefinitionsFile string `flag:"check-definitions-file,c" default:"/etc/elb-instance-status.yml" description:"File containing checks to perform for instance health"`
		UnhealthyThreshold   int64  `flag:"unhealthy-threshold" default:"5" description:"How often does a check have to fail to mark the machine unhealthy"`
		Listen               string `flag:"listen" default:":3000" description:"IP/Port to listen on for ELB health checks"`
		VersionAndExit       bool   `flag:"version" default:"false" description:"Print version and exit"`
	}{}

	version = "dev"

	checks               = []checkCommand{}
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
	rawChecks, err := ioutil.ReadFile(cfg.CheckDefinitionsFile)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(rawChecks, &checks)
}

func main() {
	if err := loadChecks(); err != nil {
		log.Fatalf("Unable to read definitions file: %s", err)
	}

	c := cron.New()
	c.AddFunc("@every 1m", spawnChecks)
	c.Start()

	spawnChecks()

	r := mux.NewRouter()
	r.HandleFunc("/status", handleELBHealthCheck)
	http.ListenAndServe(cfg.Listen, r)
}

func spawnChecks() {
	for i := range checks {
		go executeAndRegisterCheck(i)
	}
}

func executeAndRegisterCheck(checkIndex int) {
	check := checks[checkIndex]

	cmd := exec.Command("/bin/bash", "-c", check.Command)
	err := cmd.Run()

	success := err == nil

	checkResultsLock.Lock()

	if _, ok := checkResults[check.Name]; !ok {
		checkResults[check.Name] = &checkResult{
			Check: check,
		}
	}

	if success == checkResults[check.Name].IsSuccess {
		checkResults[check.Name].Streak++
	} else {
		checkResults[check.Name].IsSuccess = success
		checkResults[check.Name].Streak = 1
	}

	lastResultRegistered = time.Now()

	checkResultsLock.Unlock()
}

func handleELBHealthCheck(res http.ResponseWriter, r *http.Request) {
	healthy := true
	start := time.Now()
	buf := bytes.NewBuffer([]byte{})

	checkResultsLock.RLock()
	for cn, cr := range checkResults {
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
		fmt.Fprintf(buf, "[%s] %s\n", state, cn)
	}
	checkResultsLock.RUnlock()

	res.Header().Set("X-Collection-Parsed-In", strconv.FormatInt(time.Since(start).Nanoseconds()/int64(time.Microsecond), 10)+"ms")
	res.Header().Set("X-Last-Result-Registered-At", lastResultRegistered.Format(time.RFC1123))
	if healthy {
		res.WriteHeader(http.StatusOK)
	} else {
		res.WriteHeader(http.StatusInternalServerError)
	}

	io.Copy(res, buf)
}
