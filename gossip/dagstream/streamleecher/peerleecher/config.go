package peerleecher

import (
	"time"

	"github.com/skyhighblockchain/push-base/inter/dag"
)

type EpochDownloaderConfig struct {
	RecheckInterval        time.Duration
	DefaultChunkSize       dag.Metric
	ParallelChunksDownload int
}
