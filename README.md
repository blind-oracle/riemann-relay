# riemann-relay
This is a service that receives a Riemann Protobuf-formatted event stream and sends it to a one or more targets in Riemann or Graphite format.

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
