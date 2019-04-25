package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func initHTTP() (err error) {
	http.HandleFunc("/stats", httpStats)
	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(cfg.ListenHTTP, nil))
	return
}

func httpStats(w http.ResponseWriter, r *http.Request) {
	stats := fmt.Sprintf("Listener: %s\n", lis.getStats())

	for _, o := range outputs {
		for _, r := range strings.Split(strings.TrimSpace(o.getStats()), "\n") {
			stats += fmt.Sprintf("Output %s: %s\n", o.name, r)
		}
	}

	fmt.Fprint(w, stats)
}
