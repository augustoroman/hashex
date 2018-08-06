package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHashTask(t *testing.T) {
	defer func() { time_Sleep = time.Sleep }() // Restore time_Sleep after this test.
	var sleepAmount time.Duration
	time_Sleep = func(dt time.Duration) { sleepAmount = dt }

	t.Run("gives the CPU five seconds to plan it's strategy", func(t *testing.T) {
		HashTask("xyz").Run()
		if sleepAmount != 5*time.Second {
			t.Errorf("Hash task sleep the right amount: %v", sleepAmount)
		}
	})
	t.Run("computes the base64-encoded sha512 hash as string", func(t *testing.T) {
		const (
			input    = "angryMonkey"
			expected = `ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q==`
		)

		res, err := HashTask(input).Run()
		if err != nil {
			t.Fatal(err)
		}
		strval, ok := res.(string)
		if !ok {
			t.Fatalf("HashTask result is not a string, it's a %T: %#v", res, res)
		} else if strval != expected {
			t.Errorf("Wrong output:\nHave: %#q\nWant: %#q", strval, expected)
		}
	})
}

func TestHashApi(t *testing.T) {
	defer func() { time_Sleep = time.Sleep }() // Restore time_Sleep after this test.
	time_Sleep = func(dt time.Duration) {}     // don't make tests take 5 sec.

	t.Run("Start", func(t *testing.T) {
		t.Run("returns incrementing ids", func(t *testing.T) {
			api := &HashApi{}
			input := strings.NewReader("password=foobar")
			w, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/hash", input)
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			api.Start(w, r)
			if w.Code != 202 || w.Body.String() != "1" {
				t.Fatalf("Wrong output: status=%d body=%s", w.Code, w.Body.String())
			}

			input = strings.NewReader("password=foobar")
			w, r = httptest.NewRecorder(), httptest.NewRequest("POST", "/hash", input)
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			api.Start(w, r)
			if w.Code != 202 || w.Body.String() != "2" {
				t.Fatalf("Wrong output: status=%d body=%#q", w.Code, w.Body.String())
			}
		})
		t.Run("fails if password form field is not provided", func(t *testing.T) {
			w, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/hash", nil)
			(&HashApi{}).Start(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatal("Did not fail for a missing password param")
			}
		})
		t.Run("fails when shutting down", func(t *testing.T) {
			input := strings.NewReader("password=foobar")
			w, r := httptest.NewRecorder(), httptest.NewRequest("POST", "/hash", input)
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			api := &HashApi{}
			api.Tasks.Shutdown(context.Background())
			api.Start(w, r)
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("Did not fail after shutdown: status=%d body=%s", w.Code, w.Body.String())
			}
		})
	})

	t.Run("GetResult", func(t *testing.T) {
		t.Run("returns the hash of the input", func(t *testing.T) {
			api := &HashApi{}
			api.Tasks.Start(HashTask("angryMonkey"))
			w, r := httptest.NewRecorder(), httptest.NewRequest("GET", "/hash/1", nil)
			api.GetResult(w, r)
			const expected = `"ZEHhWB65gUlzdVwtDQArEyx+KVLzp/aTaRaPlBzYRIFj6vjFdqEb0Q5B8zVKCZ0vKbZPZklJz0Fd7su2A+gf7Q=="`
			if w.Code != 200 || w.Body.String() != expected+"\n" {
				t.Errorf("Wrong output: status=%d body=%#q", w.Code, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Wrong content type: %s", ct)
			}
		})
		// ... etc etc ...
	})
}
