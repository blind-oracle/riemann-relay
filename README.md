# riemann-relay
Service that receives a Riemann Protobuf-formatted event stream and sends it to a one or more servers in Riemann or Graphite format.

Features:
* Receive event batches in Riemann Protobuf format (see *riemann.proto*) on multiple IPs/Ports, currently TCP only
* Convert Riemann events to Carbon metrics using flexible field mapping syntax
* Send events in configurable batch sizes to any count of Riemann/Carbon targets
* Different target selection algorithms:
  - Round-Robin
  - Hash
  - Failover
  - Broadcast
* Prometheus metrics
* Configurable batch and buffer sizes, flush intervals, timeouts etc

See *riemann-relay.toml* for more details.
