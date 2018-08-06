package main

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/augustoroman/hashex/task"
)

// time_Sleep is called indirectly for a quick-and-dirty testing solution.
// This works well when time.* usage is infrequent and testing requirements
// are minimal, which fits this situation. More complicated time stuff should
// use a fake clock API.
var time_Sleep = time.Sleep

// HashTask is the task.Interface implementation for the HashApi tasks. The
// result is a string that is the sha512 hash of the string, base64-encoded.
type HashTask string

// Run executes the task and satisfies the task.Interface API.
func (h HashTask) Run() (interface{}, error) {
	time_Sleep(5 * time.Second)
	// sha512 for passwords? that's atypical.
	bin := sha512.Sum512([]byte(h))
	return base64.StdEncoding.EncodeToString(bin[:]), nil
}

// Compile-time assertion that this satisfies the task.Interface API. This is
// also enforced by it's usage with the task manager in the HashApi below.
var _ task.Interface = HashTask("")

// HashApi provides the api for hashing passwords:
//   Start()     = POST /hash     --> response is the task id
//   GetResult() = GET /hash/:id  --> response is the base64 sha512 hash
//
// HashApi is intended to be the HTTP handling front-end to task.Manager and
// HashTask, so business logic does not belong here -- only API stuff.
type HashApi struct {
	// Depending on the complexity of the tests, I might prefer to put an
	// interface here to make testing easier. But currently putting the actual
	// implementation is fine.
	Tasks task.Manager
}

// Start is the API endpoint to start a new hash operation. The password to hash
// is delivered via the POST form value 'password'. The hash operation is
// started and the operation id is returned as a string.
func (h *HashApi) Start(w http.ResponseWriter, r *http.Request) {
	// Normally, a fancier mux would take care of this.
	if r.Method != "POST" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// TODO(aroman) Auth checks here?

	// Input size limited to ~10 MB by default:
	// https://golang.org/pkg/net/http/#Request.ParseForm
	password := r.FormValue("password")
	if password == "" {
		http.Error(w, "Missing password form field", http.StatusBadRequest)
		return
	}
	// TODO(aroman) Enforce other password requirements here?

	id, err := h.Tasks.Start(HashTask(password))
	if err == task.ErrShuttingDown {
		http.Error(w, "Unable to accept new requests: the server is shutting down.",
			http.StatusServiceUnavailable)
		return
	} else if err != nil {
		log.Printf("ERROR: Attempting to start new hash: %v", err)
		// Don't send internal errors to clients... unless it's an
		// internal-only service.
		http.Error(w, "Sorry, something went wrong.", http.StatusInternalServerError)
		return
	}

	// Yay! The task was started. Use 200 OK here? Maybe 202 Accepted?
	w.WriteHeader(http.StatusAccepted)
	// OCD REST fanatics might suggest returning the full URL path for the
	// created resource: /hash/:id.  Whatever.
	io.WriteString(w, string(id))
}

// GetResult is the API endpoint to retrieve a hashed password via the
// previously-provided task id.
//
// Currently, requests to this endpoint block until the hash is complete. It
// could, alternatively, provide a short context expiration and return an
// intermediate status code suggesting that it's not ready yet... but what
// status code is that?  Maybe 102 (StatusProcessing)?
//
// https://softwareengineering.stackexchange.com/questions/316208/http-status-code-for-still-processing
// https://stackoverflow.com/questions/9794696/how-do-i-choose-a-http-status-code-in-rest-api-for-not-ready-yet-try-again-lat
func (h *HashApi) GetResult(w http.ResponseWriter, r *http.Request) {
	// Normally, a fancier mux would take care of this and id param extraction.
	if r.Method != "GET" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	id := task.Id(strings.TrimPrefix(r.URL.Path, "/hash/"))
	// TODO(aroman) id validation here?

	// TODO(aroman) Auth checks here?

	// Here we provide r.Context() which will wait around as long as the request
	// is connected. If we want different semantics, we could provide a very
	// short timeout here and, if the wait times out, then return a "it's still
	// working, please come back later" response.
	result, err := h.Tasks.Wait(r.Context(), id)
	if err == task.ErrNoSuchTask {
		http.Error(w, "No such task", http.StatusNotFound)
		return
	} else if err == context.DeadlineExceeded || err == context.Canceled {
		// The request went away. We don't really expect anyone to be listening
		// to our error response.
		http.Error(w, "Request failed, please try again.", http.StatusRequestTimeout)
		return
	} else if err != nil {
		// TODO(aroman) Can handle task-specific errors here, which may involve
		// sending error messages to the response.
		log.Printf("ERROR: Failure waiting for task %#q: %v", id, err)
		http.Error(w, "Sorry, something went wrong.", http.StatusInternalServerError)
		return
	}

	// For the hash api, we expect the result to always be a human-readable
	// string that we can write to the output. For other tasks, we'd probably
	// want more careful inspection of the result. JSON-encoding could fail if
	// the result is non-encodable, but we'll ignore that here. It's more likely
	// to fail if the client disconnects before we finish writing our response,
	// which we don't really care about.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
