package state

import (
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

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

type txSpecificStateDB struct {
	lock *sync.Mutex

	txContext *txStateContext
	*StateDB
}

func NewTxSpecificStateDB(stateDB *StateDB, sharedLock *sync.Mutex, txHash common.Hash, txIndex int) StateDBInterface {
	return &txSpecificStateDB{
		lock:    sharedLock,
		StateDB: stateDB,
		txContext: &txStateContext{
			journalTracker: journalTracker{
				journal: newJournal(),
			},
			accessList: newAccessList(),
			thash:      txHash,
			txIndex:    txIndex,
		},
	}
}

func (txDB *txSpecificStateDB) CreateAccount(addr common.Address) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.CreateAccount(addr)
}

func (txDB *txSpecificStateDB) SubBalance(addr common.Address, amount *big.Int) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.SubBalance(addr, amount)
}
func (txDB *txSpecificStateDB) AddBalance(addr common.Address, amount *big.Int) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.AddBalance(addr, amount)
}
func (txDB *txSpecificStateDB) GetBalance(addr common.Address) *big.Int {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetBalance(addr)
}

func (txDB *txSpecificStateDB) GetNonce(addr common.Address) uint64 {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetNonce(addr)
}
func (txDB *txSpecificStateDB) SetNonce(addr common.Address, nonce uint64) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.SetNonce(addr, nonce)
}

func (txDB *txSpecificStateDB) GetCodeHash(addr common.Address) common.Hash {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetCodeHash(addr)
}
func (txDB *txSpecificStateDB) GetCode(addr common.Address) []byte {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetCode(addr)
}
func (txDB *txSpecificStateDB) SetCode(addr common.Address, code []byte) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.SetCode(addr, code)
}
func (txDB *txSpecificStateDB) GetCodeSize(addr common.Address) int {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetCodeSize(addr)
}

func (txDB *txSpecificStateDB) AddRefund(amount uint64) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.AddRefund(amount)
}
func (txDB *txSpecificStateDB) SubRefund(amount uint64) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.SubRefund(amount)
}
func (txDB *txSpecificStateDB) GetRefund() uint64 {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetRefund()
}

func (txDB *txSpecificStateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetCommittedState(addr, hash)
}
func (txDB *txSpecificStateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.GetState(addr, hash)
}
func (txDB *txSpecificStateDB) SetState(addr common.Address, key common.Hash, value common.Hash) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.SetState(addr, key, value)
}

func (txDB *txSpecificStateDB) Suicide(addr common.Address) bool {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.Suicide(addr)
}
func (txDB *txSpecificStateDB) HasSuicided(addr common.Address) bool {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.HasSuicided(addr)
}

// Exist reports whether the given account exists in state.
// Notably this should also return true for suicided accounts.
func (txDB *txSpecificStateDB) Exist(addr common.Address) bool {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.Exist(addr)
}

// Empty returns whether the given account is empty. Empty
// is defined according to EIP161 (balance = nonce = code = 0).
func (txDB *txSpecificStateDB) Empty(addr common.Address) bool {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.Empty(addr)
}

func (txDB *txSpecificStateDB) PrepareAccessList(sender common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.PrepareAccessList(sender, dest, precompiles, txAccesses)
}

func (txDB *txSpecificStateDB) AddressInAccessList(addr common.Address) bool {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.AddressInAccessList(addr)
}

func (txDB *txSpecificStateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.SlotInAccessList(addr, slot)
}

// AddAddressToAccessList adds the given address to the access list. This operation is safe to perform
// even if the feature/fork is not active yet
func (txDB *txSpecificStateDB) AddAddressToAccessList(addr common.Address) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.AddAddressToAccessList(addr)
}

// AddSlotToAccessList adds the given (address,slot) to the access list. This operation is safe to perform {
// even if the feature/fork is not active yet
func (txDB *txSpecificStateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.AddSlotToAccessList(addr, slot)
}

func (txDB *txSpecificStateDB) RevertToSnapshot(snapshot int) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.RevertToSnapshot(snapshot)
}
func (txDB *txSpecificStateDB) Snapshot() int {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.Snapshot()
}

func (txDB *txSpecificStateDB) AddLog(log *types.Log) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.AddLog(log)
}
func (txDB *txSpecificStateDB) AddPreimage(hash common.Hash, preimage []byte) {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	txDB.StateDB.AddPreimage(hash, preimage)
}

func (txDB *txSpecificStateDB) ForEachStorage(addr common.Address, f func(common.Hash, common.Hash) bool) error {
	txDB.lock.Lock()
	defer txDB.lock.Unlock()

	txDB.StateDB.txStateContext = txDB.txContext
	return txDB.StateDB.ForEachStorage(addr, f)
}
