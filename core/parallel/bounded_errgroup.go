// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errgroup provides synchronization, error propagation, and Context
// cancelation for groups of goroutines working on subtasks of a common task.
package parallel

import (
	"sync"
)

// A Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
// A zero Group is valid and does not cancel on error.
type BoundedGroup struct {
	workerWG sync.WaitGroup
	workers  chan struct{}
	tasks    chan func() // A fixed size channel of
	closer   chan struct{}
}

func NewBoundedErrGroup(numWorkers int, maxPendingTasks int) *BoundedGroup {
	res := &BoundedGroup{
		workers: make(chan struct{}, numWorkers),
		tasks:   make(chan func(), maxPendingTasks),
		closer:  make(chan struct{}),
	}
	//start the numWorker worker threads
	for i := 0; i < numWorkers; i++ {
		res.workerWG.Add(1)
		go res.startWorker()
	}
	return res
}

func (g *BoundedGroup) Go(f func()) {
	// Add [f] to the task queue
	g.tasks <- f
}

func (g *BoundedGroup) startWorker() {
	defer g.workerWG.Done()

	for {
		select {
		// If the group has been marked as closed, exit.
		case <-g.closer:
			return
		case f := <-g.tasks:
			f()
		}
	}
}

func (g *BoundedGroup) Wait() {
	// Shut down the worker threads
	close(g.closer)
	g.workerWG.Wait()
}
