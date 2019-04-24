package main

type outputAlgo int

const (
	outputAlgoHash outputAlgo = iota
	outputAlgoFailover
	outputAlgoRoundRobin
)

var (
	outputAlgoMap = map[string]outputAlgo{
		"hash":       outputAlgoHash,
		"failover":   outputAlgoFailover,
		"roundrobin": outputAlgoRoundRobin,
	}
)
