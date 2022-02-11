package main

import (
	"log"
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	checkPassing       *prometheus.GaugeVec
	checkExecutionTime *prometheus.SummaryVec
	currentStatusCode  prometheus.Gauge

	dynamicLabels = []string{"check_id"}
)

func init() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to determine own hostname: %s", err)
	}

	co := prometheus.GaugeOpts{
		Subsystem:   "elb_instance_status",
		ConstLabels: prometheus.Labels{"hostname": hostname},
	}

	co.Name = "check_passing"
	co.Help = "Bit showing whether the check PASSed (=1) or FAILed (=0), WARNs are also reported as FAILs"

	cp := prometheus.NewGaugeVec(co, dynamicLabels)

	co.Name = "status_code"
	co.Help = "Contains the current HTTP status code the ELB is seeing"

	csc := prometheus.NewGauge(co)

	cet := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:   co.Namespace,
		Subsystem:   co.Subsystem,
		ConstLabels: co.ConstLabels,
		Name:        "check_execution_time",
		Help:        "Timespan in µs the execution of the check took",
	}, dynamicLabels)

	if err := prometheus.Register(cp); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			checkPassing = are.ExistingCollector.(*prometheus.GaugeVec)
		} else {
			panic(err)
		}
	}

	if err := prometheus.Register(csc); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			currentStatusCode = are.ExistingCollector.(prometheus.Gauge)
		} else {
			panic(err)
		}
	}

	if err := prometheus.Register(cet); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			checkExecutionTime = are.ExistingCollector.(*prometheus.SummaryVec)
		} else {
			panic(err)
		}
	}
}
