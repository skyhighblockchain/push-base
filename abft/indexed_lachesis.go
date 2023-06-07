package abft

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/skyhighblockchain/push-base/abft/dagidx"
	"github.com/skyhighblockchain/push-base/hash"
	"github.com/skyhighblockchain/push-base/inter/dag"
	"github.com/skyhighblockchain/push-base/inter/idx"
	"github.com/skyhighblockchain/push-base/inter/pos"
	"github.com/skyhighblockchain/push-base/kvdb"
	"github.com/skyhighblockchain/push-base/push"
)

var _ push.Consensus = (*IndexedPush)(nil)

// IndexedPush performs events ordering and detects cheaters
// It's a wrapper around Orderer, which adds features which might potentially be application-specific:
// confirmed events traversal, DAG index updates and cheaters detection.
// Use this structure if need a general-purpose consensus. Instead, use lower-level abft.Orderer.
type IndexedPush struct {
	*Push
	dagIndexer    DagIndexer
	uniqueDirtyID uniqueID
}

type DagIndexer interface {
	dagidx.VectorClock
	dagidx.ForklessCause

	Add(dag.Event) error
	Flush()
	DropNotFlushed()

	Reset(validators *pos.Validators, db kvdb.Store, getEvent func(hash.Event) dag.Event)
}

// New creates IndexedPush instance.
func NewIndexedPush(store *Store, input EventSource, dagIndexer DagIndexer, crit func(error), config Config) *IndexedPush {
	p := &IndexedPush{
		Push:          NewPush(store, input, dagIndexer, crit, config),
		dagIndexer:    dagIndexer,
		uniqueDirtyID: uniqueID{new(big.Int)},
	}

	return p
}

// Build fills consensus-related fields: Frame, IsRoot
// returns error if event should be dropped
func (p *IndexedPush) Build(e dag.MutableEvent) error {
	e.SetID(p.uniqueDirtyID.sample())

	defer p.dagIndexer.DropNotFlushed()
	err := p.dagIndexer.Add(e)
	if err != nil {
		return err
	}

	return p.Push.Build(e)
}

// Process takes event into processing.
// Event order matter: parents first.
// All the event checkers must be launched.
// Process is not safe for concurrent use.
func (p *IndexedPush) Process(e dag.Event) (err error) {
	defer p.dagIndexer.DropNotFlushed()
	err = p.dagIndexer.Add(e)
	if err != nil {
		return err
	}

	err = p.Push.Process(e)
	if err != nil {
		return err
	}
	p.dagIndexer.Flush()
	return nil
}

func (p *IndexedPush) Bootstrap(callback push.ConsensusCallbacks) error {
	base := p.Push.OrdererCallbacks()
	ordererCallbacks := OrdererCallbacks{
		ApplyAtropos: base.ApplyAtropos,
		EpochDBLoaded: func(epoch idx.Epoch) {
			if base.EpochDBLoaded != nil {
				base.EpochDBLoaded(epoch)
			}
			p.dagIndexer.Reset(p.store.GetValidators(), p.store.epochTable.VectorIndex, p.input.GetEvent)
		},
	}
	return p.Push.BootstrapWithOrderer(callback, ordererCallbacks)
}

type uniqueID struct {
	counter *big.Int
}

func (u *uniqueID) sample() [24]byte {
	u.counter = u.counter.Add(u.counter, common.Big1)
	var id [24]byte
	copy(id[:], u.counter.Bytes())
	return id
}
