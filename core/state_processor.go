// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/parallel"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/sync/errgroup"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *BlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	if cfg.RequireAccessList {
		return p.processParallel(block, statedb, cfg)
	} else {
		return p.processSync(block, statedb, cfg)
	}
}

// processSync is the existing implementation from Ethereum for processing transactions serially. It has not and should
// not be changed from the original geth implementation.
func (p *StateProcessor) processSync(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts    types.Receipts
		usedGas     = new(uint64)
		header      = block.Header()
		blockHash   = block.Hash()
		blockNumber = block.Number()
		allLogs     []*types.Log
		gp          = new(GasPool).AddGas(block.GasLimit())
	)
	// Mutate the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}
	blockContext := NewEVMBlockContext(header, p.bc, nil)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, p.config, cfg)
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		msg, err := tx.AsMessage(types.MakeSigner(p.config, header.Number), header.BaseFee)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		statedb.Prepare(tx.Hash(), i)
		receipt, err := applyTransaction(msg, p.config, p.bc, nil, gp, statedb, blockNumber, blockHash, tx, usedGas, vmenv)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}
	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles())

	return receipts, allLogs, *usedGas, nil
}

// processParallel attempts to process the transactiosn in [block] in parallel by wrapping everything with concurrent safe data
// structures and forcing transactions to grab locks to access the state that they wish to use.
func (p *StateProcessor) processParallel(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	var (
		receipts    types.Receipts = make([]*types.Receipt, len(block.Transactions()))
		usedGas                    = new(uint64)
		header                     = block.Header()
		blockHash                  = block.Hash()
		blockNumber                = block.Number()
		txLogs                     = make([][]*types.Log, len(block.Transactions()))
		allLogs     []*types.Log
		gp          = new(GasPool).AddGas(block.GasLimit())
		sharedLock  = &sync.Mutex{}
	)
	// Mutate the block and state according to any hard-fork specs
	if p.config.DAOForkSupport && p.config.DAOForkBlock != nil && p.config.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(statedb)
	}
	blockContext := NewEVMBlockContext(header, p.bc, nil)

	txLocker := parallel.NewAccessListLocker(block.Transactions())

	var eg errgroup.Group
	// Iterate over and process the individual transactions
	for i, tx := range block.Transactions() {
		// Create closure with i and tx, so that the loop does not overwrite the memory used in
		// the goroutine.
		i := i
		tx := tx
		eg.Go(func() error {
			log.Info(fmt.Sprintf("starting goroutine for tx (%s, %d)", tx.Hash(), i))
			// Grab the locks for every item in the access list. This will block until the transaction
			// can acquire all the necessary locks.
			txLocker.Lock(tx)
			log.Info(fmt.Sprintf("successfully grabbed locks for tx (%s, %d)", tx.Hash(), i))

			txDB := state.NewTxSpecificStateDB(statedb, sharedLock, tx.Hash(), i)
			vmenv := vm.NewEVM(blockContext, vm.TxContext{}, txDB, p.config, cfg)
			msg, err := tx.AsMessage(types.MakeSigner(p.config, header.Number), header.BaseFee)
			if err != nil {
				return fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
			}
			receipt, err := applyTransaction(msg, p.config, p.bc, nil, gp, txDB, blockNumber, blockHash, tx, usedGas, vmenv)
			if err != nil {
				return fmt.Errorf("could not apply tx %d [%v]: %w", i, tx.Hash().Hex(), err)
			}
			log.Info(fmt.Sprintf("releasing locks for tx (%s, %d)", tx.Hash(), i))
			txLocker.Unlock(tx)
			log.Info(fmt.Sprintf("released locks for tx (%s, %d)", tx.Hash(), i))
			// Set the receipt and logs at the correct index
			receipts[i] = receipt
			txLogs[i] = receipt.Logs
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, 0, err
	}
	// Coalesce the logs
	for _, logs := range txLogs {
		allLogs = append(allLogs, logs...)
	}
	// Update the tx receipt cumulative gas used values and then make sure that the total gas used
	// is below the block gas limit.
	// Assuming the remaining gas in the gas pool does not impact the EVM execution unless it hits the block
	// gas limit, this should work fine.
	// For any block that doesn't hit the gas limit this should be fine, but if a block does hit the gas limit
	// it may be non-deterministic which transaction reverts in this case. In that case, one option would be to
	// fallback to executing transactions serially.
	// XXX this is currently a non-deterministic bug in the parallel execution path.
	var cumulativeGasUsed uint64
	for _, receipt := range receipts {
		cumulativeGasUsed += receipt.GasUsed
		receipt.CumulativeGasUsed = cumulativeGasUsed
	}

	if cumulativeGasUsed > header.GasLimit {
		return nil, nil, 0, fmt.Errorf("block exceeded gas limit (%d) with (%d)", header.GasLimit, cumulativeGasUsed)
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	p.engine.Finalize(p.bc, header, statedb, block.Transactions(), block.Uncles())

	return receipts, allLogs, *usedGas, nil
}

func applyTransaction(msg types.Message, config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb state.StateDBInterface, blockNumber *big.Int, blockHash common.Hash, tx *types.Transaction, usedGas *uint64, evm *vm.EVM) (*types.Receipt, error) {
	// Create a new context to be used in the EVM environment.
	txContext := NewEVMTxContext(msg)
	evm.Reset(txContext, statedb)

	// Apply the transaction to the current state (included in the env).
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, err
	}

	// Update the state with pending changes.
	// var root []byte
	if config.IsByzantium(blockNumber) {
		statedb.Finalise(true)
	} else {
		statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
		// root = statedb.IntermediateRoot(config.IsEIP158(blockNumber)).Bytes()
	}
	atomic.AddUint64(usedGas, result.UsedGas)

	// Create a new receipt for the transaction, storing the intermediate root and gas used
	// by the tx.
	receipt := &types.Receipt{Type: tx.Type(), CumulativeGasUsed: atomic.LoadUint64(usedGas)}
	if result.Failed() {
		receipt.Status = types.ReceiptStatusFailed
	} else {
		receipt.Status = types.ReceiptStatusSuccessful
	}
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas

	// If the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(evm.TxContext.Origin, tx.Nonce())
	}

	// Set the receipt logs and create the bloom filter.
	receipt.Logs = statedb.GetLogs(tx.Hash(), blockHash)
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.BlockHash = blockHash
	receipt.BlockNumber = blockNumber
	receipt.TransactionIndex = uint(statedb.TxIndex())
	return receipt, err
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, error) {
	msg, err := tx.AsMessage(types.MakeSigner(config, header.Number), header.BaseFee)
	if err != nil {
		return nil, err
	}
	// Create a new context to be used in the EVM environment
	blockContext := NewEVMBlockContext(header, bc, author)
	vmenv := vm.NewEVM(blockContext, vm.TxContext{}, statedb, config, cfg)
	return applyTransaction(msg, config, bc, author, gp, statedb, header.Number, header.Hash(), tx, usedGas, vmenv)
}
