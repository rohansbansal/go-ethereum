package parallel

import (
	"golang.org/x/sync/errgroup"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type TxExecutor struct {
	txs []*types.Transaction

	addressLocks map[common.Address]*FIFOLocker
	execute      func(tx *types.Transaction) error
}

// TODO:
// need to make the statedb and evm safe to use in parallel

func NewTxExecutor(txs []*types.Transaction, execute func(tx *types.Transaction) error) *TxExecutor {
	executor := &TxExecutor{
		txs:          txs,
		addressLocks: make(map[common.Address]*FIFOLocker),
	}

	// Construct the executor
	for _, tx := range txs {
		for _, accessTuple := range tx.AccessList() {
			locker, exists := executor.addressLocks[accessTuple.Address]
			if !exists {
				executor.addressLocks[accessTuple.Address] = NewFIFOLocker(tx.Hash())
			} else {
				locker.Reserve(tx.Hash())
			}
		}
	}

	return executor
}

func (e *TxExecutor) Execute() error {
	var wg errgroup.Group
	for _, tx := range e.txs {
		// Create local variable so [tx] is not overwritten on the next iteration of the loop
		tx := tx
		wg.Go(func() error {
			// Grab the locks for every item in the access list. This will block until the transaction
			// can acquire all the necessary locks.
			for _, accessTuple := range tx.AccessList() {
				locker := e.addressLocks[accessTuple.Address]
				locker.Lock(tx.Hash())
			}
			return e.execute(tx)
		})
	}

	return wg.Wait()
}
