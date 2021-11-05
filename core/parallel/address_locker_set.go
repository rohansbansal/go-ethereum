package parallel

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type AccessListLocker struct {
	addressLocks map[common.Address]*FIFOLocker
}

func NewAccessListLocker(txs []*types.Transaction) *AccessListLocker {
	al := &AccessListLocker{
		addressLocks: make(map[common.Address]*FIFOLocker),
	}

	for _, tx := range txs {
		for _, accessTuple := range tx.AccessList() {
			if lock, exists := al.addressLocks[accessTuple.Address]; exists {
				lock.Reserve(tx.Hash())
			} else {
				al.addressLocks[accessTuple.Address] = NewFIFOLocker(tx.Hash())
			}
		}
	}
	return al
}

func (a *AccessListLocker) Lock(tx *types.Transaction) {
	for _, accessTuple := range tx.AccessList() {
		lock := a.addressLocks[accessTuple.Address]
		lock.Lock(tx.Hash())
	}
}

func (a *AccessListLocker) Unlock(tx *types.Transaction) {
	for _, accessTuple := range tx.AccessList() {
		lock := a.addressLocks[accessTuple.Address]
		lock.Unlock(tx.Hash())
	}
}
