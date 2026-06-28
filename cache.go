package main

import (
	"strings"
	"sync"
	"time"
)

type CacheService struct {
	mu                sync.RWMutex
	cache             map[string]WeatherResponse
	expirationMinutes int64
}

func NewCacheService() *CacheService {
	return &CacheService{
		cache:             make(map[string]WeatherResponse),
		expirationMinutes: 30,
	}
}

func (cs *CacheService) isExpired(timestamp int64) bool {
	return (time.Now().UnixMilli() - timestamp) > (cs.expirationMinutes * 60 * 1000)
}

func (cs *CacheService) Get(city string) (WeatherResponse, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	key := strings.ToLower(city)
	cached, found := cs.cache[key]
	if found && !cs.isExpired(cached.Timestamp) {
		return cached, true
	}
	return WeatherResponse{}, false
}

func (cs *CacheService) Put(city string, response WeatherResponse) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := strings.ToLower(city)
	cs.cache[key] = response
}