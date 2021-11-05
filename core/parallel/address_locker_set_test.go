package parallel

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/sync/errgroup"
)

func TestTxLocker(t *testing.T) {
	addrs := make([]common.Address, 0)
	for i := 0; i < 10; i++ {
		addrs = append(addrs, common.Address{byte(i)})
	}
	txs := []*types.Transaction{
		types.NewTx(&types.DynamicFeeTx{
			Nonce: 0,
			AccessList: []types.AccessTuple{
				{
					Address: addrs[0],
				},
				{
					Address: addrs[1],
				},
				{
					Address: addrs[2],
				},
			},
		}),
		types.NewTx(&types.DynamicFeeTx{
			Nonce: 1,
			AccessList: []types.AccessTuple{
				{
					Address: addrs[0],
				},
				{
					Address: addrs[2],
				},
			},
		}),
		types.NewTx(&types.DynamicFeeTx{
			Nonce: 2,
			AccessList: []types.AccessTuple{
				{
					Address: addrs[3],
				},
				{
					Address: addrs[1],
				},
			},
		}),
		types.NewTx(&types.DynamicFeeTx{
			Nonce: 3,
			AccessList: []types.AccessTuple{
				{
					Address: addrs[4],
				},
				{
					Address: addrs[1],
				},
			},
		}),
		types.NewTx(&types.DynamicFeeTx{
			Nonce: 4,
			AccessList: []types.AccessTuple{
				{
					Address: addrs[6],
				},
				{
					Address: addrs[1],
				},
			},
		}),
		types.NewTx(&types.DynamicFeeTx{
			Nonce: 5,
			AccessList: []types.AccessTuple{
				{
					Address: addrs[7],
				},
				{
					Address: addrs[1],
				},
			},
		}),
	}

	lock := NewAccessListLocker(txs)
	var eg errgroup.Group
	for _, tx := range txs {
		tx := tx
		eg.Go(func() error {
			lock.Lock(tx)
			lock.Unlock(tx)

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatal(err)
	}
}
