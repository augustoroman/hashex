package task

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"testing"
)

type trackRunsTask int32
type failTask string
type syncTask chan string

func (t *trackRunsTask) Run() (interface{}, error) {
	atomic.AddInt32((*int32)(t), 1)
	return "done", nil
}
func (f failTask) Run() (interface{}, error) {
	return nil, errors.New(string(f))
}
func (t syncTask) Run() (interface{}, error) {
	t <- "started!"
	return <-t, nil
}

// Probably should actually split these up.
func TestManager(t *testing.T) {
	t.Run("Start", func(t *testing.T) {
		t.Run("returns sequential ids", func(t *testing.T) {
			var task trackRunsTask
			var tm Manager

			if id, err := tm.Start(&task); err != nil {
				t.Fatal(err)
			} else if id != "1" {
				t.Fatalf("Wrong id:%#q", id)
			}

			if id, err := tm.Start(&task); err != nil {
				t.Fatal(err)
			} else if id != "2" {
				t.Fatalf("Wrong id:%#q", id)
			}

			if id, err := tm.Start(&task); err != nil {
				t.Fatal(err)
			} else if id != "3" {
				t.Fatalf("Wrong id:%#q", id)
			}
		})
		t.Run("Runs the tasks", func(t *testing.T) {
			task := syncTask(make(chan string))
			var tm Manager
			tm.Start(task)
			select {
			case <-task:
				task <- "finish"
			case <-time.After(time.Second):
				t.Fatal("task did not run within a second!")
			}
		})
		// TODO: Test fails on shutdown
	})
	t.Run("Wait", func(t *testing.T) {
		t.Run("returns the result of the task", func(t *testing.T) {
			var task1 trackRunsTask
			var task2 = failTask("oops")
			var tm Manager

			tm.Start(&task1)
			tm.Start(task2)

			if res, err := tm.Wait(context.Background(), "1"); err != nil {
				t.Fatal(err)
			} else if res != "done" {
				t.Errorf("Wrong output: %#v", res)
			} else if int(task1) != 1 {
				t.Errorf("Task 1 didn't run 1 time? %d", task1)
			}

			if res, err := tm.Wait(context.Background(), "2"); err == nil {
				t.Fatalf("Expected an error, but got none: res=%#v err=%v", res, err)
			} else if err.Error() != "oops" {
				t.Errorf("Wrong error: %#v", err)
			}
		})
		t.Run("waits for the task to complete", func(t *testing.T) {
			task := syncTask(make(chan string))
			var tm Manager
			tm.Start(task)

			done := make(chan string)
			go func() {
				res, err := tm.Wait(context.Background(), "1")
				if err != nil {
					t.Fatal(err)
				} else if res != "go" {
					t.Errorf("Wrong output: %#q", res)
				}
				close(done)
			}()

			assertRecvWithin(t, task, "started!", time.Second)
			assertNoRecvWithin(t, done, 50*time.Millisecond)
			task <- "go"
			assertRecvWithin(t, done, "", time.Second)
		})
		t.Run("can be interrupted by the context", func(t *testing.T) {
			task := syncTask(make(chan string))
			var tm Manager
			tm.Start(task)

			ctx, cancel := context.WithCancel(context.Background())

			done := make(chan string)
			go func() {
				res, err := tm.Wait(ctx, "1")
				if err != context.Canceled {
					t.Errorf("Wrong output: res=%#v err=%v", res, err)
				}
				close(done)
			}()

			// Give up to a second for things that should happen instantly since
			// in a real CI env it can take a "long" time for a heavily loaded
			// machine. One sec is enough that even a machine under load
			// should be able to manage a couple channel operations in.
			assertRecvWithin(t, task, "started!", time.Second)
			// When we expect nothing to happen, we don't want the tests to just
			// sit there. 50ms is enough that it _might_ accidentally pass under
			// a very heavily loaded test machine, but the vast majority of time
			// it'll fail if there's a bug.
			assertNoRecvWithin(t, done, 50*time.Millisecond)
			cancel()
			assertRecvWithin(t, done, "", time.Second)
		})
		// TODO: test ErrNoSuchTask
	})
	// TODO: Test shutdown
}

func assertRecvWithin(t *testing.T, ch chan string, expected string, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	select {
	case val := <-ch:
		// Cool, we got something before timing out. Was it the right thing?
		if val != expected {
			t.Fatalf("Received %#q instead of %#q after %v",
				val, expected, time.Since(start))
		}
	case <-time.After(timeout):
		t.Fatalf("Timed out (%v) waiting for %#q", timeout, expected)
	}
}

func assertNoRecvWithin(t *testing.T, ch chan string, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	select {
	case val := <-ch:
		t.Fatalf("Received %#q after %v, expected nothing within %v",
			val, time.Since(start), timeout)
	case <-time.After(timeout):
		// good, we timed out
	}
}
