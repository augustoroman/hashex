package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// EndPointStatsTracker tracks the performance of one or more http.HandlerFuncs.
// It implements http.Handler that reports the collected statistics.
//
// NOTE(aroman) An alternative API would be to have this just be a stats
// collecter and return the stats via an accessor, and define the handler
// separately. This would be nice if we wanted to use the stats more generally.
type EndPointStatsTracker struct {
	// TODO(aroman) this type would be more useful if this was a
	// map[string]callStats and Track took a string identifier:
	//   Track(name string, f http.HandlerFunc) http.HandlerFunc
	// and then ServeHTTP would provide metrics on several endpoints.
	stats callStats
	mutex sync.Mutex
}

// Track wraps an http.HandlerFunc to provide a HandlerFunc that tracks the
// performance of that func.
func (e *EndPointStatsTracker) Track(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h(w, r)
		elapsed := time.Since(start)

		e.mutex.Lock()
		e.stats.Add(elapsed)
		e.mutex.Unlock()
	}
}

// ServeHTTP responds to the http request with the collected statistics.
func (e *EndPointStatsTracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mutex.Lock()
	stats := e.stats
	e.mutex.Unlock()

	// Reformat the stats to correspond to the desired API.
	apiStats := struct {
		Total       int `json:"total"`
		AverageUSec int `json:"average"`
	}{
		Total:       stats.NumCalls,
		AverageUSec: int(stats.Average() / time.Microsecond),
	}
	// We don't care about encoding errors -- the only possible errors here are
	// write errors if the client disconnects early.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(apiStats)
}

// callStats represents the collected statistics for a particular endpoint.
type callStats struct {
	NumCalls int
	Elapsed  time.Duration
}

// Average returns the average duration per call, or 0 if there is no data yet.
func (c callStats) Average() time.Duration {
	if c.NumCalls == 0 {
		return 0
	}
	return c.Elapsed / time.Duration(c.NumCalls)
}

// Add accumulates the duration of a new call into this object.
func (c *callStats) Add(e time.Duration) {
	c.NumCalls++
	c.Elapsed += e
}
