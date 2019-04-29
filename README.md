[![Go Report Card](https://goreportcard.com/badge/github.com/blind-oracle/riemann-relay)](https://goreportcard.com/report/github.com/blind-oracle/riemann-relay)
[![Coverage Status](https://coveralls.io/repos/github/blind-oracle/riemann-relay/badge.svg?branch=master)](https://coveralls.io/github/blind-oracle/riemann-relay?branch=master)
[![Build Status](https://travis-ci.org/blind-oracle/riemann-relay.svg?branch=master)](https://travis-ci.org/blind-oracle/riemann-relay)
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge-flat.svg)](https://github.com/avelino/awesome-go)

# riemann-relay
This is a service that receives a Riemann Protobuf-formatted event stream and sends it to a one or more targets in Riemann or Graphite format.
Although that can be done in Riemann itself, this service is simpler, probably faster and lightweight (no Java)

## Features
* Receive event batches in Riemann Protobuf format (see *riemann.proto*) on multiple IPs/Ports, currently TCP only
* Convert Riemann events to Carbon metrics using flexible field mapping syntax
* Send events in configurable batch sizes to any number of Riemann/Carbon targets
* Different target selection algorithms:
  - Round-Robin
  - Hash
  - Failover
  - Broadcast
* Prometheus metrics
* Log stats periodically
* Configurable batch and buffer sizes, flush intervals, timeouts

See *riemann-relay.toml* for more details on these features and how to configure them

## Performance
On 2 average CPU cores it's able to handle about 500k events per second, depending on batch size and incoming riemann message sizes.
It will scale to more CPUs when using more targets and clients (each target and client gets it's own thread).
There's a room for optimizations, though.

## Build
**riemann-relay** is written in Go and uses Dep as a dependency manager, so you need to install them first.

Then:
```bash
# dep ensure
# go build
```

## Run
```bash
# /path/to/riemann-relay -config /etc/riemann-relay.toml
```

The logging currently goes to stdout.
Use *-debug* option to get a lot more detailed output (not for production).
