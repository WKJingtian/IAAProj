package idgen

import (
	"fmt"
	"sync"
	"time"
)

const (
	timestampBits  = 41
	generatorBits  = 10
	sequenceBits   = 12
	MaxGeneratorID = (1 << generatorBits) - 1
	maxSequence    = (1 << sequenceBits) - 1
	timestampShift = generatorBits + sequenceBits
	generatorShift = sequenceBits
	DefaultEpochMS = int64(1735689600000)
)

type SnowflakeConfig struct {
	GeneratorID uint16
	EpochMS     int64
	Now         func() time.Time
}

type SnowflakeGenerator struct {
	mu          sync.Mutex
	generatorID uint16
	epochMS     int64
	lastUnixMS  int64
	sequence    uint16
	now         func() time.Time
}

func NewSnowflakeGenerator(cfg SnowflakeConfig) (*SnowflakeGenerator, error) {
	if cfg.GeneratorID > MaxGeneratorID {
		return nil, fmt.Errorf("generator id %d exceeds max %d", cfg.GeneratorID, MaxGeneratorID)
	}
	if cfg.EpochMS == 0 {
		cfg.EpochMS = DefaultEpochMS
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	return &SnowflakeGenerator{
		generatorID: cfg.GeneratorID,
		epochMS:     cfg.EpochMS,
		now:         cfg.Now,
	}, nil
}

func (g *SnowflakeGenerator) NextID() (uint64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	nowUnixMS := g.currentUnixMS()
	if nowUnixMS < g.lastUnixMS {
		nowUnixMS = g.waitUntil(g.lastUnixMS)
		if nowUnixMS < g.lastUnixMS {
			return 0, fmt.Errorf("clock moved backwards: last=%d current=%d", g.lastUnixMS, nowUnixMS)
		}
	}
	if nowUnixMS < g.epochMS {
		return 0, fmt.Errorf("current time %d is before epoch %d", nowUnixMS, g.epochMS)
	}

	if nowUnixMS == g.lastUnixMS {
		if g.sequence >= maxSequence {
			nowUnixMS = g.waitUntil(g.lastUnixMS + 1)
			g.sequence = 0
		} else {
			g.sequence++
		}
	} else {
		g.sequence = 0
	}

	timestampPart := uint64(nowUnixMS - g.epochMS)
	if timestampPart >= (uint64(1) << timestampBits) {
		return 0, fmt.Errorf("timestamp overflow: %d", timestampPart)
	}

	g.lastUnixMS = nowUnixMS
	return (timestampPart << timestampShift) | (uint64(g.generatorID) << generatorShift) | uint64(g.sequence), nil
}

func (g *SnowflakeGenerator) currentUnixMS() int64 {
	return g.now().UTC().UnixMilli()
}

func (g *SnowflakeGenerator) waitUntil(targetUnixMS int64) int64 {
	for {
		nowUnixMS := g.currentUnixMS()
		if nowUnixMS >= targetUnixMS {
			return nowUnixMS
		}
		time.Sleep(time.Millisecond)
	}
}
