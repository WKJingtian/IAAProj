package localcache

import (
	"context"
	"sync"
	"time"

	"common/applog"
)

const (
	defaultWriteBackTTL           = 10 * time.Minute
	defaultWriteBackFlushInterval = 50 * time.Second
)

type WriteBackCache[T any] struct {
	ttl           time.Duration
	flushInterval time.Duration
	flusher       func(context.Context, string, T) error

	mu      sync.Mutex
	entries map[string]*writeBackEntry[T]

	stopCh    chan struct{}
	stoppedCh chan struct{}
	stopOnce  sync.Once
}

type writeBackEntry[T any] struct {
	data      T
	dirty     bool
	expiresAt time.Time
	version   uint64
}

type writeBackSnapshot[T any] struct {
	key     string
	data    T
	version uint64
}

func NewWriteBackCache[T any](ttl time.Duration, flushInterval time.Duration, flusher func(context.Context, string, T) error) *WriteBackCache[T] {
	if ttl <= 0 {
		ttl = defaultWriteBackTTL
	}
	if flushInterval <= 0 {
		flushInterval = defaultWriteBackFlushInterval
	}
	if flusher == nil {
		flusher = func(context.Context, string, T) error { return nil }
	}

	c := &WriteBackCache[T]{
		ttl:           ttl,
		flushInterval: flushInterval,
		flusher:       flusher,
		entries:       make(map[string]*writeBackEntry[T]),
		stopCh:        make(chan struct{}),
		stoppedCh:     make(chan struct{}),
	}

	go c.runFlushLoop()
	return c
}

func (c *WriteBackCache[T]) Get(key string, now time.Time) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		var zero T
		return zero, false
	}

	entry.expiresAt = now.Add(c.ttl)
	return entry.data, true
}

func (c *WriteBackCache[T]) StoreLoaded(key string, data T, now time.Time) T {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok {
		entry.expiresAt = now.Add(c.ttl)
		return entry.data
	}

	c.entries[key] = &writeBackEntry[T]{
		data:      data,
		expiresAt: now.Add(c.ttl),
	}
	return data
}

func (c *WriteBackCache[T]) Replace(key string, data T, now time.Time) T {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok {
		entry.data = data
		entry.expiresAt = now.Add(c.ttl)
		return entry.data
	}

	c.entries[key] = &writeBackEntry[T]{
		data:      data,
		expiresAt: now.Add(c.ttl),
	}
	return data
}

func (c *WriteBackCache[T]) MutateIfPresent(key string, now time.Time, mutate func(*T) (bool, error)) (T, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		var zero T
		return zero, false, nil
	}

	candidate := entry.data
	changed, err := mutate(&candidate)
	if err != nil {
		entry.expiresAt = now.Add(c.ttl)
		return entry.data, true, err
	}
	if changed {
		entry.data = candidate
		entry.dirty = true
		entry.version++
	}
	entry.expiresAt = now.Add(c.ttl)
	return entry.data, true, nil
}

func (c *WriteBackCache[T]) MutateWithLoaded(key string, loaded T, now time.Time, mutate func(*T) (bool, error)) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok {
		candidate := entry.data
		changed, err := mutate(&candidate)
		if err != nil {
			entry.expiresAt = now.Add(c.ttl)
			return entry.data, err
		}
		if changed {
			entry.data = candidate
			entry.dirty = true
			entry.version++
		}
		entry.expiresAt = now.Add(c.ttl)
		return entry.data, nil
	}

	changed, err := mutate(&loaded)
	if err != nil {
		return loaded, err
	}
	c.entries[key] = &writeBackEntry[T]{
		data:      loaded,
		dirty:     changed,
		expiresAt: now.Add(c.ttl),
		version:   versionForChange(changed),
	}
	return loaded, nil
}

func (c *WriteBackCache[T]) FlushNow(ctx context.Context) error {
	snapshots := c.collectFlushable(time.Now().UTC())
	if len(snapshots) == 0 {
		return nil
	}

	for _, snapshot := range snapshots {
		if err := c.flusher(ctx, snapshot.key, snapshot.data); err != nil {
			return err
		}
		c.markFlushed(snapshot.key, snapshot.version)
	}

	return nil
}

func (c *WriteBackCache[T]) Close(ctx context.Context) error {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})

	select {
	case <-c.stoppedCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	return c.FlushNow(ctx)
}

func (c *WriteBackCache[T]) runFlushLoop() {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()
	defer close(c.stoppedCh)

	for {
		select {
		case <-ticker.C:
			flushCtx, cancel := context.WithTimeout(context.Background(), c.flushInterval)
			if err := c.FlushNow(flushCtx); err != nil {
				applog.Errorf("flush write-back cache failed: %v", err)
			}
			cancel()
		case <-c.stopCh:
			return
		}
	}
}

func (c *WriteBackCache[T]) collectFlushable(now time.Time) []writeBackSnapshot[T] {
	c.mu.Lock()
	defer c.mu.Unlock()

	snapshots := make([]writeBackSnapshot[T], 0)
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) && !entry.dirty {
			delete(c.entries, key)
			continue
		}
		if !entry.dirty {
			continue
		}

		snapshots = append(snapshots, writeBackSnapshot[T]{
			key:     key,
			data:    entry.data,
			version: entry.version,
		})
	}

	return snapshots
}

func (c *WriteBackCache[T]) markFlushed(key string, version uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return
	}
	if entry.version != version || !entry.dirty {
		return
	}

	entry.dirty = false
	if time.Now().UTC().After(entry.expiresAt) {
		delete(c.entries, key)
	}
}

func versionForChange(changed bool) uint64 {
	if changed {
		return 1
	}
	return 0
}
