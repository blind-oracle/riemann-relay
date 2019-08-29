package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

func initHTTP() {
	httpLis, err := listen(cfg.ListenHTTP)
	if err != nil {
		log.Fatalf("Unable to listen to HTTP: %s", err)
	}

	http.HandleFunc("/stats", httpStats)
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.Serve(httpLis, nil))
}

func httpStats(w http.ResponseWriter, r *http.Request) {
	out := fmt.Sprintf("riemann-relay v%s up for %s\n", version, time.Since(startTime))

	for _, n := range inputNames {
		i := inputs[n]
		out += fmt.Sprintf("Input %s:\n", n)

		for _, r := range strings.Split(i.getStats(), "\n") {
			out += fmt.Sprintf(" %s\n", r)
		}
	}

	for _, n := range outputNames {
		o := outputs[n]
		out += fmt.Sprintf("Output %s:\n", n)

		for _, r := range o.getStats() {
			out += fmt.Sprintf(" %s\n", r)
		}
	}

	fmt.Fprint(w, out)
}
