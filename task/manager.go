// Package task implements a simple asynchronous task management scheme.
package task

import (
	"context"
	"errors"
	"strconv"
	"sync"
)

// Interface is the common interface implemented for a task that can be managed
// by a Manager.
//
// Run executes the task and returns the result and/or an error.
type Interface interface {
	Run() (interface{}, error)
}

// Id identifies a task to a manager.
type Id string

// Manager keeps track of a set of tasks. Currently, it keeps tasks forever but
// it should have a way of expiring tasks.
type Manager struct {
	mutex    sync.Mutex
	tasks    map[Id]*taskOutput
	stopping bool

	running sync.WaitGroup
}
type taskOutput struct {
	done   chan struct{}
	result interface{}
	err    error
}

var (
	ErrShuttingDown = errors.New("shutting down: cannot start a new task")
	ErrNoSuchTask   = errors.New("no such task")
)

// Start initiates the execution of the provided task and returns the id. If
// Shutdown has been called, then this will return ErrShuttingDown.
func (tm *Manager) Start(task Interface) (Id, error) {
	tm.mutex.Lock()
	if tm.stopping {
		tm.mutex.Unlock()
		return "", ErrShuttingDown
	}
	if tm.tasks == nil {
		tm.tasks = map[Id]*taskOutput{}
	}
	nextId := Id(strconv.Itoa(len(tm.tasks) + 1))
	ti := &taskOutput{done: make(chan struct{})}
	tm.tasks[nextId] = ti
	tm.running.Add(1)
	tm.mutex.Unlock()

	go func() {
		ti.result, ti.err = task.Run()
		close(ti.done)
		tm.running.Done()
	}()

	return nextId, nil
}

// Wait for the given task to be completed and return the result & error output
// of the task. Once a task completes, subsequent calls to this function will
// immediately return the outputs. If the provided context finishes before the
// task has completed, then the context error (cancelled or timeout) will be
// returned.
//
// NOTE(aroman) Probably this should only be allowed to be called once
// succesfully (that is, not including the context timeout) and then expire the
// task to prevent excessive memory growth.
func (tm *Manager) Wait(ctx context.Context, id Id) (interface{}, error) {
	tm.mutex.Lock()
	ti := tm.tasks[id]
	tm.mutex.Unlock()

	if ti == nil {
		return nil, ErrNoSuchTask
	}

	// TODO(aroman) Consider the semantics around shutting down. Currently this
	// allows tasks to complete, but maybe that should be configurable? It's
	// already possible to control depending on the underlying task
	// implementation, but that puts more of a burden on the task writer.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ti.done:
		// TODO(aroman) Depending on the desired semantics, we should probably
		// mark the task as expirable now to avoid excessively collecting
		// memory.
		return ti.result, ti.err
	}
}

// Shutdown disallows new tasks from being started and waits until the existing
// tasks all complete. This returns an error only if the provided context is
// done before all the tasks have completed.
func (tm *Manager) Shutdown(ctx context.Context) error {
	tm.mutex.Lock()
	tm.stopping = true
	tm.mutex.Unlock()

	// Allow the sync.WaitGroup to be select-able.
	allDone := make(chan bool)
	go func() {
		tm.running.Wait()
		close(allDone)
	}()

	select {
	case <-allDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
