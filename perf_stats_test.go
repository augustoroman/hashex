package main

import "testing"

func TestEndPointStatsTracker(t *testing.T) {
	// things to test:
	// - that it's totally safe when accessed concurrently (run with go test -race too)
	//   (both for the wrapped Track handler and the ServeHTTP call)
	// - use a bunch of channels in the handlers to maximize contention
	//     (e.g. see https://godoc.org/github.com/fluxio/sync_testing that I wrote at Flux)
	// - check that there's no divide-by-0
	// - replace time_Since and time_Now calls with indirect version to validate
	//   time operations... or use a fake clock, or do some heuristics of dt > X.
}
