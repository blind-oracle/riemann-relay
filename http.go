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
	stats := fmt.Sprintf("Listener: %s\n", lis.getStats())

	for _, o := range outputs {
		for _, r := range strings.Split(strings.TrimSpace(o.getStats()), "\n") {
			stats += fmt.Sprintf("Output %s: %s\n", o.name, r)
		}
	}

	fmt.Fprint(w, stats)
}
