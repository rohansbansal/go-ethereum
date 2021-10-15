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

func init() {
	gopath := os.Getenv("GOPATH")
	counterSrc, err := filepath.Abs(gopath + "/src/github.com/ava-labs/coreth/examples/counter/counter.sol")
	if err != nil {
		panic(err)
	}
	contracts, err := compiler.CompileSolidity("", counterSrc)
	if err != nil {
		panic(err)
	}
	contract, _ := contracts[fmt.Sprintf("%s:%s", counterSrc, "Counter")]
	code = common.Hex2Bytes(contract.Code[2:])
}

// Benchmarks large blocks with value transfers to non-existing accounts
func benchmarkArbitraryBlockExecution(b *testing.B, numBlocks int, blockGenerator func(int, *BlockGen), requireAccessList bool) {
	// Generate the original common chain segment and the two competing forks
	engine := ethash.NewFaker()
	db := rawdb.NewMemoryDatabase()
	genesis := gspec.MustCommit(db)

	shared, _ := GenerateChain(gspec.Config, genesis, engine, db, numBlocks, blockGenerator)
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
		if _, err := chain.InsertChain(shared); err != nil {
			b.Fatalf("failed to insert shared chain: %v", err)
		}
		b.StopTimer()
	}
}

func BenchmarkSimpleBlockTransactionExecution(b *testing.B) {
	numTxs := 1000
	benchmarkArbitraryBlockExecution(b, 10, func(i int, block *BlockGen) {
		block.SetCoinbase(common.Address{1})

		for txi := 0; txi < numTxs; txi++ {
			uniq := uint64(i*numTxs + txi)
			tx, err := types.SignTx(types.NewTransaction(uniq, testBankAddress, big.NewInt(1), params.TxGas, big.NewInt(1), nil), signer, testBankKey)
			if err != nil {
				b.Error(err)
			}
			block.AddTx(tx)
		}
	}, false)
}
