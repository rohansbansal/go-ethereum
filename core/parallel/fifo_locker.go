package parallel

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type FIFOLocker struct {
	lock sync.Mutex

	headTx  common.Hash
	txQueue []common.Hash

	txLocks map[common.Hash]*sync.Mutex
}

// NewFIFOLocker creates a new FIFO locker with [head]
// as the first reserver. Ie. [head] must release before
// anyone else can access the resource.
func NewFIFOLocker(head common.Hash) *FIFOLocker {
	return &FIFOLocker{
		headTx:  head,
		txQueue: make([]common.Hash, 0),
		txLocks: make(map[common.Hash]*sync.Mutex),
	}
}

func (f *FIFOLocker) Reserve(txHash common.Hash) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.txQueue = append(f.txQueue, txHash)
	// Create a lock and grab it immediately. This must be unlocked by the
	// previous item in the queue, before the lock can be grabbed.
	lock := &sync.Mutex{}
	lock.Lock()
	f.txLocks[txHash] = lock
}

func (f *FIFOLocker) Lock(txHash common.Hash) {
	f.lock.Lock()

	// Allow [headTx] to execute immediately without
	// grabbing any new locks
	if f.headTx == txHash {
		f.lock.Unlock()
		return
	}

	lock, exists := f.txLocks[txHash]
	if !exists {
		f.lock.Unlock()
		panic(fmt.Sprintf("unexpected attempt to grab lock from txHash: %s", txHash))
	}
	f.lock.Unlock()
	lock.Lock()
}

func (f *FIFOLocker) Unlock(txHash common.Hash) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.headTx != txHash {
		panic(fmt.Sprintf("unlock attempt from incorrect tx hash: %s", txHash))
	}

	// If there are no more transactions to unlock, then return immediately
	if len(f.txQueue) == 0 {
		return
	}
	// Extract the next transaction and update the txQueue
	f.headTx, f.txQueue = f.txQueue[0], f.txQueue[0:]
	// Unlock the lock corresponding to the updated [f.headTx], so that the goroutine
	// that is blocking attempting to grab the lock will be released.
	lock, exists := f.txLocks[f.headTx]
	if !exists {
		panic(fmt.Sprintf("failed to find lock for txHash: %s", f.headTx))
	}
	lock.Unlock()
}
