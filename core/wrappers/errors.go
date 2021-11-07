package wrappers

import (
	"sync"
)

type Errs struct {
	Err error
	rw  *sync.RWMutex
}

func (errs *Errs) Errored() bool {
	errs.rw.RLock()
	defer errs.rw.RUnlock()
	return errs.Err != nil
}

func (errs *Errs) Add(errors ...error) {
	errs.rw.RLock()
	defer errs.rw.RUnlock()
	if errs.Err == nil {
		for _, err := range errors {
			if err != nil {
				errs.rw.RUnlock()
				errs.rw.Lock()
				errs.Err = err
				errs.rw.Unlock()
				break
			}
		}
	}
}
