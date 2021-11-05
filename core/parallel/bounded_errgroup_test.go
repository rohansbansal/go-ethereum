// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parallel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// JustErrors illustrates the use of a Group in place of a sync.WaitGroup to
// simplify goroutine counting and error handling. This example is derived from
// the sync.WaitGroup example at https://golang.org/pkg/sync/#example_WaitGroup.
func ExampleBoundedGroup_justErrors() {
	g := NewBoundedErrGroup(2, 3)
	var urls = []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	for _, url := range urls {
		// Launch a goroutine to fetch the URL.
		url := url // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			// Fetch the URL.
			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
			}
			return err
		})
	}
	// Wait for all HTTP fetches to complete.
	if err := g.Wait(); err == nil {
		fmt.Println("Successfully fetched all URLs.")
	}
}

func TestBoundedGroupWaitOnNothing(t *testing.T) {
	g := NewBoundedErrGroup(10, 10)

	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestBoundedGroupTasks(t *testing.T) {
	g := NewBoundedErrGroup(10, 10)

	errVal := errors.New("fail")
	errs := []error{
		errVal,
		// nil,
		// nil,
		// nil,
		// nil,
	}

	for _, err := range errs {
		err := err
		g.Go(func() error { return err })
	}

	err := g.Wait()
	if err != errVal {
		t.Fatalf("Expected error %s to match %s", err, errVal)
	}
}

func TestZeroBoundedGroup(t *testing.T) {
	err1 := errors.New("errgroup_test: 1")
	err2 := errors.New("errgroup_test: 2")

	cases := []struct {
		errs []error
	}{
		{errs: []error{}},
		{errs: []error{nil}},
		{errs: []error{err1}},
		{errs: []error{err1, nil}},
		{errs: []error{err1, nil, err2}},
	}

	for _, tc := range cases {
		var firstErr error

		g := NewBoundedErrGroup(10, 10)
		for _, err := range tc.errs {
			err := err
			g.Go(func() error { return err })

			if firstErr == nil && err != nil {
				firstErr = err
			}
		}
		if gErr := g.Wait(); gErr != firstErr {
			t.Fatalf("Expected error: %s, found error: %s", firstErr, gErr)
		}
	}
}

func TestBoundedWithContext(t *testing.T) {
	errDoom := errors.New("group_test: doomed")

	cases := []struct {
		errs []error
		want error
	}{
		{want: nil},
		{errs: []error{nil}, want: nil},
		{errs: []error{errDoom}, want: errDoom},
		{errs: []error{errDoom, nil}, want: errDoom},
	}

	for _, tc := range cases {
		g, ctx := NewBoundedErrGroupWithContext(context.Background(), 10, 10)

		for _, err := range tc.errs {
			err := err
			g.Go(func() error { return err })
		}

		if err := g.Wait(); err != tc.want {
			t.Errorf("after %T.Go(func() error { return err }) for err in %v\n"+
				"g.Wait() = %v; want %v",
				g, tc.errs, err, tc.want)
		}

		canceled := false
		select {
		case <-ctx.Done():
			canceled = true
		default:
		}
		if !canceled {
			t.Errorf("after %T.Go(func() error { return err }) for err in %v\n"+
				"ctx.Done() was not closed",
				g, tc.errs)
		}
	}
}
