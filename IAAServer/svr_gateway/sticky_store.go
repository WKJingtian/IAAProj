package main

import (
	"strings"
	"time"

	"common/localcache"
)

const stickyBindingKeyPrefix = "gateway:sticky:game:"

type stickyStore struct {
	cache *localcache.TimedCache
	ttl   time.Duration
}

func newStickyStore(ttl time.Duration) *stickyStore {
	return &stickyStore{
		cache: localcache.New(localcache.Config{}),
		ttl:   ttl,
	}
}

func (s *stickyStore) Get(openid string) (stickyBinding, bool, error) {
	var binding stickyBinding

	ok, err := s.cache.GetJSON(s.key(openid), &binding)
	if err != nil {
		return stickyBinding{}, false, err
	}
	if !ok {
		return stickyBinding{}, false, nil
	}
	return binding, true, nil
}

func (s *stickyStore) Set(openid string, binding stickyBinding) error {
	return s.cache.SetJSON(s.key(openid), binding, s.ttl)
}

func (s *stickyStore) Delete(openid string) {
	s.cache.Delete(s.key(openid))
}

func (s *stickyStore) Close() {
	s.cache.Close()
}

func (s *stickyStore) key(openid string) string {
	return stickyBindingKeyPrefix + strings.TrimSpace(openid)
}
