package localcache

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

const defaultSweepInterval = time.Minute

var ErrEmptyKey = errors.New("local cache key cannot be empty")

type KV interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration) error
	Delete(keys ...string) int
	Expire(key string, ttl time.Duration) bool
	TTL(key string) (time.Duration, bool)
	Close()
}

type Config struct {
	SweepInterval time.Duration
}

type TimedCache struct {
	mu            sync.Mutex
	values        map[string]entry
	sweepInterval time.Duration
	stopCh        chan struct{}
	stoppedCh     chan struct{}
	closeOnce     sync.Once
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

func New(config Config) *TimedCache {
	sweepInterval := config.SweepInterval
	if sweepInterval <= 0 {
		sweepInterval = defaultSweepInterval
	}

	c := &TimedCache{
		values:        make(map[string]entry),
		sweepInterval: sweepInterval,
		stopCh:        make(chan struct{}),
		stoppedCh:     make(chan struct{}),
	}

	go c.runSweepLoop()
	return c
}

func (c *TimedCache) Get(key string) ([]byte, bool) {
	if key == "" {
		return nil, false
	}

	now := time.Now().UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	stored, ok := c.values[key]
	if !ok {
		return nil, false
	}
	if isExpired(stored, now) {
		delete(c.values, key)
		return nil, false
	}

	return cloneBytes(stored.value), true
}

func (c *TimedCache) Set(key string, value []byte, ttl time.Duration) error {
	if key == "" {
		return ErrEmptyKey
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.values[key] = entry{
		value:     cloneBytes(value),
		expiresAt: expiresAtFromTTL(ttl),
	}
	return nil
}

func (c *TimedCache) Delete(keys ...string) int {
	now := time.Now().UTC()
	removed := 0

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, key := range keys {
		if key == "" {
			continue
		}
		stored, ok := c.values[key]
		if !ok {
			continue
		}
		if isExpired(stored, now) {
			delete(c.values, key)
			continue
		}
		delete(c.values, key)
		removed++
	}

	return removed
}

func (c *TimedCache) Expire(key string, ttl time.Duration) bool {
	if key == "" {
		return false
	}

	now := time.Now().UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	stored, ok := c.values[key]
	if !ok {
		return false
	}
	if isExpired(stored, now) {
		delete(c.values, key)
		return false
	}

	stored.expiresAt = expiresAtFromTTL(ttl)
	c.values[key] = stored
	return true
}

func (c *TimedCache) TTL(key string) (time.Duration, bool) {
	if key == "" {
		return 0, false
	}

	now := time.Now().UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	stored, ok := c.values[key]
	if !ok {
		return 0, false
	}
	if isExpired(stored, now) {
		delete(c.values, key)
		return 0, false
	}
	if stored.expiresAt.IsZero() {
		return -1, true
	}

	return time.Until(stored.expiresAt), true
}

func (c *TimedCache) SetJSON(key string, value any, ttl time.Duration) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(key, encoded, ttl)
}

func (c *TimedCache) GetJSON(key string, dest any) (bool, error) {
	raw, ok := c.Get(key)
	if !ok {
		return false, nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (c *TimedCache) Close() {
	c.closeOnce.Do(func() {
		close(c.stopCh)
		<-c.stoppedCh
	})
}

func (c *TimedCache) runSweepLoop() {
	ticker := time.NewTicker(c.sweepInterval)
	defer ticker.Stop()
	defer close(c.stoppedCh)

	for {
		select {
		case <-ticker.C:
			c.sweepExpired(time.Now().UTC())
		case <-c.stopCh:
			return
		}
	}
}

func (c *TimedCache) sweepExpired(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, stored := range c.values {
		if isExpired(stored, now) {
			delete(c.values, key)
		}
	}
}

func cloneBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}

	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func expiresAtFromTTL(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().UTC().Add(ttl)
}

func isExpired(stored entry, now time.Time) bool {
	return !stored.expiresAt.IsZero() && now.After(stored.expiresAt)
}
