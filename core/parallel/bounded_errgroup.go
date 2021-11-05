// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errgroup provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.
package parallel

import (
	"context"
	"sync"
)

// A Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
// A zero Group is valid and does not cancel on error.
type BoundedGroup struct {
	cancel func()

	workerWG, taskWG sync.WaitGroup
	workers          chan struct{}
	tasks            chan func() error // A fixed size channel of
	closer           chan struct{}

	errOnce sync.Once
	err     error
}

func NewBoundedErrGroupWithContext(parentCtx context.Context, numWorkers int, maxPendingTasks int) (*BoundedGroup, context.Context) {
	bg := NewBoundedErrGroup(numWorkers, maxPendingTasks)
	ctx, cancel := context.WithCancel(parentCtx)
	bg.cancel = cancel
	return bg, ctx
}

func NewBoundedErrGroup(numWorkers int, maxPendingTasks int) *BoundedGroup {
	return &BoundedGroup{
		workers: make(chan struct{}, numWorkers),
		tasks:   make(chan func() error, maxPendingTasks),
		closer:  make(chan struct{}),
	}
}

func (g *BoundedGroup) Go(f func() error) {
	// Start an additional worker, if we have not yet reached the worker thread cap.
	select {
	case g.workers <- struct{}{}:
		g.workerWG.Add(1)
		go g.startWorker()
	default:
	}

	// Add [f] to the task queue
	g.taskWG.Add(1)
	g.tasks <- func() error {
		defer g.taskWG.Done()
		return f()
	}
}

func (g *BoundedGroup) startWorker() {
	defer g.workerWG.Done()

	for {
		select {
		// If the group has been marked as closed, exit.
		case <-g.closer:
			return
		case f := <-g.tasks:
			if err := f(); err != nil {
				g.errOnce.Do(func() {
					g.err = err
					if g.cancel != nil {
						g.cancel()
					}
				})
			}
		}
	}
}

func (g *BoundedGroup) Wait() error {
	// Wait for all tasks to finish
	g.taskWG.Wait()

	// Shut down the worker threads
	close(g.closer)
	g.workerWG.Wait()

	// Call [cancel] if supplied.
	if g.cancel != nil {
		g.cancel()
	}

	// Return the appropriate error
	return g.err
}
