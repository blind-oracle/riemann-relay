package main

import (
	"fmt"
	"net/http"
	"strings"

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
	var out string

	for _, i := range inputs {
		for _, r := range strings.Split(strings.TrimSpace(i.getStats()), "\n") {
			out += fmt.Sprintf("Input %s: %s\n", i.name, r)
		}
	}

	for _, o := range outputs {
		for _, r := range strings.Split(strings.TrimSpace(o.getStats()), "\n") {
			out += fmt.Sprintf("Output %s: %s\n", o.name, r)
		}
	}

	fmt.Fprint(w, out)
}
