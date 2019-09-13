package main

import "github.com/prometheus/client_golang/prometheus"

var (
	promTgtBuffered = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_buffered",
			Help: "Number of events buffered by target",
		},

		[]string{"output", "target"},
	)

	promTgtSent = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_sent",
			Help: "Number of events sent by target",
		},

		[]string{"output", "target"},
	)

	promTgtDroppedBufferFull = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_dropped_buffer_full",
			Help: "Number of events dropped by target because the buffer is full",
		},

		[]string{"output", "target"},
	)

	promTgtConnFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_conn_failed",
			Help: "Number of connection failures by output/target/connection",
		},

		[]string{"output", "target", "conn"},
	)

	promTgtFlushFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tgt_flush_failed",
			Help: "Number of flush failures by target",
		},

		[]string{"output", "target"},
	)

	promTgtFlushDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tgt_flush_duration",
			Help:    "Duration of flushes by target",
			Buckets: prometheus.LinearBuckets(0.01, 0.01, 10),
		},

		[]string{"output", "target"},
	)

	promOutReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "out_received",
			Help: "Number of events received by output",
		},

		[]string{"output"},
	)

	promOutDroppedBufferFull = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "out_dropped_buffer_full",
			Help: "Number of events dropped by output because the buffer is full",
		},

		[]string{"output"},
	)

	promOutDroppedNoTargetsAlive = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "out_no_target_alive",
			Help: "Number of events dropped because no targets alive by output",
		},

		[]string{"output"},
	)
)

func init() {
	prometheus.MustRegister(
		promTgtBuffered,
		promTgtSent,
		promTgtDroppedBufferFull,
		promTgtConnFailed,
		promTgtFlushFailed,
		promTgtFlushDuration,

		promOutReceived,
		promOutDroppedBufferFull,
		promOutDroppedNoTargetsAlive,
	)
}
