package parallel

import (
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestFIFOLocker(t *testing.T) {
	txHashes := make([]common.Hash, 0)
	for i := 0; i < 10; i++ {
		txHashes = append(txHashes, common.Hash{byte(i)})
	}

	locker := NewFIFOLocker(txHashes[0])

	// locker.Lock(txHashes[0])
	// locker.Unlock(txHashes[0])
	// locker.Lock(txHashes[1])
	// locker.Unlock(txHashes[1])

	for _, txHash := range txHashes[1:] {
		locker.Reserve(txHash)
	}

	var wg sync.WaitGroup
	for _, txHash := range txHashes {
		txHash := txHash
		wg.Add(1)
		go func() {
			defer wg.Done()

			locker.Lock(txHash)
			locker.Unlock(txHash)
		}()
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for locks to finish")
	case <-done:
	}
}

func TestFIFOLockerWithErrGroup(t *testing.T) {
	txHashes := make([]common.Hash, 0)
	for i := 0; i < 10; i++ {
		txHashes = append(txHashes, common.Hash{byte(i)})
	}

	locker := NewFIFOLocker(txHashes[0])

	// locker.Lock(txHashes[0])
	// locker.Unlock(txHashes[0])
	// locker.Lock(txHashes[1])
	// locker.Unlock(txHashes[1])

	for _, txHash := range txHashes[1:] {
		locker.Reserve(txHash)
	}

	var wg sync.WaitGroup
	for _, txHash := range txHashes {
		txHash := txHash
		wg.Add(1)
		go func() {
			defer wg.Done()

			locker.Lock(txHash)
			locker.Unlock(txHash)
		}()
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for locks to finish")
	case <-done:
	}
}
