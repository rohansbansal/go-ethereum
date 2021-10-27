// Copyright 2014 The go-ethereum Authors
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
	cryptorand "crypto/rand"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

var (
	signer          = types.HomesteadSigner{}
	testBankKey, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	bankFunds       = new(big.Int).Mul(big.NewInt(999999999999999999), big.NewInt(1000000000))
	gspec           = Genesis{
		Config: params.TestPreLondonConfig,
		Alloc: GenesisAlloc{
			testBankAddress: {Balance: bankFunds},
			common.HexToAddress("0xc0de"): {
				Code:    []byte{0x60, 0x01, 0x50},
				Balance: big.NewInt(0),
			}, // push 1, pop
		},
		GasLimit: 100e6, // 100 M
	}
)

// genRandomAddrs generates [numKeys] randomly generated private keys
func genRandomAddrs(numKeys int) []*keystore.Key {
	res := make([]*keystore.Key, numKeys)
	for i := 0; i < numKeys; i++ {
		res[i] = keystore.NewKeyForDirectICAP(cryptorand.Reader)
	}
	return res
}

// generateRanodomExecution returns the list of blocks necessary to deploy [numContracts] and send dummy transactions from [numKeys] randomized
// over which key is sending the transaction as well as which contract is being called. This will be done for [numTxs] per block across [numBlocks].
// [requireAccessList] indicates whether the vmConfig will require a complete access list.
// Also returns an int specifying the index of the first random block (as opposed to the setup blocks for deploying contracts and distributing funds).
func generateRandomExecution(b *testing.B, numContracts int, numBlocks int, numTxs int, numKeys int, requireAccessList bool) ([]*types.Block, int) {
	engine := ethash.NewFaker()
	db := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(db)
	// Create a an empty slice of blocks with enough space pre-allocated
	// for [numBlocks], plus additional space for the block deploying contracts,
	// the block sending funds to the randomly generated keys.
	// Note: blocks excludes the genesis block.
	blocks := make([]*types.Block, 0, numBlocks+2)
	gopath := os.Getenv("GOPATH")

	// generate contract from source code within the repo
	contractSrc, err := filepath.Abs(gopath + "/src/github.com/ethereum/go-ethereum/core/Storage.sol")
	if err != nil {
		b.Fatal(err)
	}
	contracts, err := compiler.CompileSolidity("", contractSrc)
	if err != nil {
		b.Fatal(err)
	}
	if err != nil {
		b.Fatal(err)
	}

	// Deploy [numContracts] instances of the Storage contract
	contractAddrs := make([]common.Address, 0, numContracts)
	contract, exists := contracts[fmt.Sprintf("%s:%s", contractSrc, "Storage")]
	if !exists {
		contract, exists = contracts["Storage.sol:Storage"]
	}
	if !exists {
		b.Fatal("contract doesn't exist")
	}
	code := common.Hex2Bytes(contract.Code[2:])
	// Generate a single block that will be responsible for deploying the contracts at the start of the execution
	generateContractsBlock, _ := GenerateChain(gspec.Config, genesis, engine, db, 1, func(i int, block *BlockGen) {
		for i := 0; i < numContracts; i++ {
			tx, err := types.SignTx(types.NewContractCreation(uint64(i), common.Big0, 1_000_000, common.Big1, code), signer, testBankKey)
			if err != nil {
				b.Fatal(err)
			}
			block.AddTx(tx)
		}
	})
	// Add the single block generated to deploy [numContracts] Storage contracts.
	blocks = append(blocks, generateContractsBlock...)

	// Create a closure here, so that we can simply throw away the database and blockchain
	// that is created temporarily in order to get the receipts from the transactions. The
	// receipts are used to get the addresses of the deployed contracts.
	{
		diskdb := rawdb.NewMemoryDatabase()
		gspec.MustCommit(diskdb)

		chain, err := NewBlockChain(diskdb, nil, gspec.Config, engine, vm.Config{
			RequireAccessList: requireAccessList,
		}, nil, nil)
		if err != nil {
			b.Fatalf("failed to create tester chain: %v", err)
		}
		if _, err := chain.InsertChain(generateContractsBlock); err != nil {
			b.Fatalf("failed to insert shared chain: %v", err)
		}
		receipts := chain.GetReceiptsByHash(chain.CurrentBlock().Hash())
		for _, receipt := range receipts {
			contractAddrs = append(contractAddrs, receipt.ContractAddress)
		}
	}

	// Generate the private keys and create a slice of a single block that will
	// be responsible for sending funds to each of the addresses.
	keys := genRandomAddrs(numKeys)
	fundPrivateKeysBlock, _ := GenerateChain(gspec.Config, blocks[len(blocks)-1], engine, db, numBlocks, func(i int, block *BlockGen) {
		block.SetCoinbase(common.Address{1})

		for _, key := range keys {
			tx, err := types.SignTx(types.NewTransaction(block.TxNonce(testBankAddress), key.Address, new(big.Int).Mul(big.NewInt(10), big.NewInt(int64(params.Ether))), params.TxGas, nil, nil), signer, testBankKey)
			if err != nil {
				b.Fatal(err)
			}
			block.AddTx(tx)
		}
	})
	blocks = append(blocks, fundPrivateKeysBlock...)

	// Generate chain of [numBlocks], containing [numTxs] each, which will create random transactions calling a random
	// storage contract from one of the randomly generated private keys.
	callDataMissingAddress := common.Hex2Bytes("6057361d000000000000000000000000")
	randomBlocks, _ := GenerateChain(gspec.Config, blocks[len(blocks)-1], engine, db, numBlocks, func(i int, block *BlockGen) {
		block.SetCoinbase(common.Address{1})
		for txi := 0; txi < numTxs; txi++ {
			key := keys[rand.Intn(len(keys))]
			modifiedCallData := append(callDataMissingAddress, key.Address.Bytes()...)
			tx, err := types.SignTx(types.NewTransaction(block.TxNonce(key.Address), contractAddrs[rand.Intn(len(contractAddrs))], big.NewInt(0), 50_000, big.NewInt(1), modifiedCallData), signer, key.PrivateKey)
			if err != nil {
				b.Error(err)
			}
			block.AddTx(tx)
		}
	})
	index := len(blocks)
	blocks = append(blocks, randomBlocks...)
	return blocks, index
}

func benchmarkRandomBlockExecution(b *testing.B, numBlocks int, numTxs int, numContracts int, numKeys int, requireAccessList bool, memdb bool) {
	// Generate the slice of blocks whose execution we wish to benchmark.
	blocks, startIndex := generateRandomExecution(b, numContracts, numBlocks, numTxs, numKeys, requireAccessList)

	for i := 0; i < b.N; i++ {
		var diskdb ethdb.Database
		if memdb {
			diskdb = rawdb.NewMemoryDatabase()
		} else {
			panic("not implemented to use actual disk databaase TODO: use leveldb")
		}
		gspec.MustCommit(diskdb)
		chain, err := NewBlockChain(diskdb, nil, gspec.Config, ethash.NewFaker(), vm.Config{
			RequireAccessList: requireAccessList,
		}, nil, nil)
		if err != nil {
			b.Fatalf("failed to create tester chain: %v", err)
		}
		if _, err := chain.InsertChain(blocks[:startIndex]); err != nil {
			b.Fatalf("failed to insert setup blocks for random execution: %v", err)
		}
		b.StartTimer()
		if _, err := chain.InsertChain(blocks[startIndex:]); err != nil {
			b.Fatalf("failed to insert shared chain: %v", err)
		}
		b.StopTimer()
	}
}

// Benchmarks large blocks with value transfers to non-existing accounts
func benchmarkArbitraryBlockExecution(b *testing.B, numBlocks int, numTxs int, requireAccessList bool) {
	// Generate the original common chain segment and the two competing forks
	engine := ethash.NewFaker()
	db := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(db)

	gopath := os.Getenv("GOPATH")
	contractSrc, err := filepath.Abs(gopath + "/src/github.com/ethereum/go-ethereum/core/Storage.sol")
	if err != nil {
		b.Fatal(err)
	}
	contracts, err := compiler.CompileSolidity("", contractSrc)
	if err != nil {
		b.Fatal(err)
	}
	numContracts := 10
	contractAddrs := make([]common.Address, 0, numContracts)
	contract, exists := contracts[fmt.Sprintf("%s:%s", contractSrc, "Storage")]
	if !exists {
		contract, exists = contracts["Storage.sol:Storage"]
	}
	if !exists {
		b.Fatal("contract doesn't exist")
	}
	code := common.Hex2Bytes(contract.Code[2:])
	generateContractChain, _ := GenerateChain(gspec.Config, genesis, engine, db, 1, func(i int, block *BlockGen) {
		for i := 0; i < numContracts; i++ {
			tx, err := types.SignTx(types.NewContractCreation(uint64(i), common.Big0, 1_000_000, common.Big1, code), signer, testBankKey)
			if err != nil {
				b.Fatal(err)
			}
			block.AddTx(tx)
		}
	})
	// Insert the block craeted by [generateContractChain] into a dummy chain first
	// to gather the created contract addresses.
	{
		// Import the genertate contract chain to get the receipts
		diskdb := rawdb.NewMemoryDatabase()
		gspec.MustCommit(diskdb)

		chain, err := NewBlockChain(diskdb, nil, gspec.Config, engine, vm.Config{
			RequireAccessList: requireAccessList,
		}, nil, nil)
		if err != nil {
			b.Fatalf("failed to create tester chain: %v", err)
		}
		if _, err := chain.InsertChain(generateContractChain); err != nil {
			b.Fatalf("failed to insert shared chain: %v", err)
		}
		receipts := chain.GetReceiptsByHash(chain.CurrentBlock().Hash())
		for _, receipt := range receipts {
			contractAddrs = append(contractAddrs, receipt.ContractAddress)
		}
	}
	//Create a storage contract 0xefc81a8c
	//a6f9dae1
	callDataMissingAddress := common.Hex2Bytes("6057361d000000000000000000000000")
	blockGenerator := func(i int, block *BlockGen) {
		block.SetCoinbase(common.Address{1})
		for txi := 0; txi < numTxs; txi++ {
			uniq := uint64(i*numTxs + txi + numContracts)
			addr := common.Address{byte(txi)}
			modifiedCallData := append(callDataMissingAddress, addr.Bytes()...)
			tx, err := types.SignTx(types.NewTransaction(uniq, contractAddrs[rand.Intn(10)], big.NewInt(1), 50_000, big.NewInt(1), modifiedCallData), signer, testBankKey)
			if err != nil {
				b.Error(err)
			}
			block.AddTx(tx)
		}
	}
	shared, _ := GenerateChain(gspec.Config, generateContractChain[0], engine, db, numBlocks, blockGenerator)
	blocks := append(generateContractChain, shared...)
	b.StopTimer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Import the shared chain and the original canonical one
		diskdb := rawdb.NewMemoryDatabase()
		gspec.MustCommit(diskdb)

		chain, err := NewBlockChain(diskdb, nil, gspec.Config, engine, vm.Config{
			RequireAccessList: requireAccessList,
		}, nil, nil)
		if err != nil {
			b.Fatalf("failed to create tester chain: %v", err)
		}
		b.StartTimer()
		if _, err := chain.InsertChain(blocks); err != nil {
			b.Fatalf("failed to insert shared chain: %v", err)
		}
		b.StopTimer()
	}
}

func BenchmarkSimpleBlockTransactionExecution(b *testing.B) {
	benchmarkArbitraryBlockExecution(b, 10, 10, false)
}

func BenchmarkRandomParallelExecution(b *testing.B) {
	benchmarkRandomBlockExecution(b, 10, 10, 10, 10, true, true)
}
