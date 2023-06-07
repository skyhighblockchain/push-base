package abft

import (
	"github.com/skyhighblockchain/push-base/inter/idx"
	"github.com/skyhighblockchain/push-base/inter/pos"
	"github.com/skyhighblockchain/push-base/kvdb"
	"github.com/skyhighblockchain/push-base/kvdb/memorydb"
	"github.com/skyhighblockchain/push-base/push"
	"github.com/skyhighblockchain/push-base/utils/adapters"
	"github.com/skyhighblockchain/push-base/vecfc"
)

type applyBlockFn func(block *push.Block) *pos.Validators

// TestPush extends Push for tests.
type TestPush struct {
	*IndexedPush

	blocks map[idx.Block]*push.Block

	applyBlock applyBlockFn
}

// FakePush creates empty abft with mem store and equal weights of nodes in genesis.
func FakePush(nodes []idx.ValidatorID, weights []pos.Weight, mods ...memorydb.Mod) (*TestPush, *Store, *EventStore) {
	validators := make(pos.ValidatorsBuilder, len(nodes))
	for i, v := range nodes {
		if weights == nil {
			validators[v] = 1
		} else {
			validators[v] = weights[i]
		}
	}

	openEDB := func(epoch idx.Epoch) kvdb.DropableStore {
		return memorydb.New()
	}
	crit := func(err error) {
		panic(err)
	}
	store := NewStore(memorydb.New(), openEDB, crit, LiteStoreConfig())

	err := store.ApplyGenesis(&Genesis{
		Validators: validators.Build(),
		Epoch:      FirstEpoch,
	})
	if err != nil {
		panic(err)
	}

	input := NewEventStore()

	config := LiteConfig()
	lch := NewIndexedPush(store, input, &adapters.VectorToDagIndexer{vecfc.NewIndex(crit, vecfc.LiteConfig())}, crit, config)

	extended := &TestPush{
		IndexedPush: lch,
		blocks:      map[idx.Block]*push.Block{},
	}

	blockIdx := idx.Block(0)

	err = extended.Bootstrap(push.ConsensusCallbacks{
		BeginBlock: func(block *push.Block) push.BlockCallbacks {
			blockIdx++
			return push.BlockCallbacks{
				EndBlock: func() (sealEpoch *pos.Validators) {
					// track blocks
					extended.blocks[blockIdx] = block
					if extended.applyBlock != nil {
						return extended.applyBlock(block)
					}
					return nil
				},
			}
		},
	})
	if err != nil {
		panic(err)
	}

	return extended, store, input
}
