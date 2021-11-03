package state

import "github.com/ethereum/go-ethereum/common"

// txStateContext is intended to contain all of the tx specific
// information that the statedb tracks, so that by swapping a single
// pointer, the statedb wrapper can tell the statedb where to look
// for all of the tx specific information.
type txStateContext struct {
	journalTracker
	accessList *accessList
	thash      common.Hash
	txIndex    int
	refund     uint64
}

// Plan:
// txStateContext tracks all of the tx specific information
// therefore, instead of using the normal statedb within the EVM,
// we will instead access it through a txSpecificStateDB intended for
// managed concurrent access to the statedb.

// This struct will grab the statedb lock, set the txStateContext
// do the computation, and release the statedb lock, so that during the
// execution of the individual function call, the statedb will operate on the correct context
