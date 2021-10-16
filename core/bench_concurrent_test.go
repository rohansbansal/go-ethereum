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
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

var (
	signer          = types.HomesteadSigner{}
	testBankKey, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	bankFunds       = big.NewInt(100000000000000000)
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
	code []byte
)

// Benchmarks large blocks with value transfers to non-existing accounts
func benchmarkArbitraryBlockExecution(b *testing.B, numBlocks int, numTxs int, requireAccessList bool) {
	// Generate the original common chain segment and the two competing forks
	engine := ethash.NewFaker()
	db := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(db)

	gopath := os.Getenv("GOPATH")
	contractSrc, err := filepath.Abs(gopath + "/src/github.com/ethereum/go-ethereum/core/Owner.sol")
	if err != nil {
		b.Fatal(err)
	}
	contracts, err := compiler.CompileSolidity("", contractSrc)
	if err != nil {
		b.Fatal(err)
	}
	numContracts := 10
	contractAddrs := make([]common.Address, 0, numContracts)
	contract, _ := contracts[fmt.Sprintf("%s:%s", contractSrc, "Owner")]
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

	callDataMissingAddress := common.Hex2Bytes("a6f9dae1000000000000000000000000")
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
