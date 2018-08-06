// hashex is an example server of an asynchronous hashing service.
//
// The overall structure of the code is broken into 4 parts:
//   1. task.Manager provides the business logic of running async tasks tracked
//      by id.
//   2. HashApi layers the desired HTTP API semantics onto the task.Manager,
//      and HashTask provides the actual hash operation.
//   3. EndPointStatsTracker implements the performance tracking, wrapping the
//      HashApi endpoint.
//   4. main() plugs everything together and handles shutdown.
//
// Graceful shutdown is done via a combination of task.Manager and main. This
// pierces the HashApi abstraction a bit. :-/
//
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	port := flag.Int("port", 8080, "Port to serve on")
	bind := flag.String("bind", "127.0.0.1", "IP to bind to for serving. An "+
		"empty value means to serve on all available interfaces. The default "+
		"value serves only on the local machine.")
	flag.Parse()

	server := &http.Server{
		Addr: net.JoinHostPort(*bind, fmt.Sprint(*port)),
		// In a real production env, also set timeouts defensively. Ref:
		//   https://blog.cloudflare.com/exposing-go-on-the-internet/
	}

	var hashApi HashApi
	var perf EndPointStatsTracker

	// I like hooking everything up in one place so you can easily see the
	// complete map of incoming requests -> handlers, even if that's 100s of
	// lines long. Also, a proper mux would allow separating out POST vs GEt
	// here rather than in the handlers.
	http.HandleFunc("/hash", perf.Track(hashApi.Start))
	http.HandleFunc("/hash/", hashApi.GetResult)
	http.HandleFunc("/stats", perf.ServeHTTP)

	http.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Shutting down")
		go server.Shutdown(context.Background())
	})

	// TODO(aroman) Prod should have consistent access logs for all endpoints.
	// TODO(aroman) Prod should have secured pprof and expvar endpoints.

	// Handle ^C cleanly. To be a good citizen, the first ^C is consumed and
	// shutdown is initiated, but any further ^Cs are handled by the OS, which
	// probably means... ☠.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		signal.Reset(os.Interrupt) // A second ^C kills the server immediately.
		server.Shutdown(context.Background())
	}()

	log.Printf("Starting hash API server on %s", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Cannot start server: %v", err)
	}

	log.Printf("Waiting for running tasks && active requests to finish.")
	ctx := context.Background() // Wait indefinitely for shutdown.
	hashApi.Tasks.Shutdown(ctx) // Wait for all tasks to finish.
	server.Shutdown(ctx)        // Wait for all in-flight requests to finish.
}
