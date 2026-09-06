package main

import (
	"strings"
	"sync"
	"time"
)

// CacheService provides concurrent-safe, in-memory caching for weather data.
type CacheService struct {
	mu                sync.RWMutex
	cache             map[string]WeatherResponse
	expirationMinutes int64
}

// NewCacheService initializes the caching layer with a default 30-minute TTL.
func NewCacheService() *CacheService {
	return &CacheService{
		cache:             make(map[string]WeatherResponse),
		expirationMinutes: 30,
	}
}

func (cs *CacheService) isExpired(timestamp int64) bool {
	return (time.Now().UnixMilli() - timestamp) > (cs.expirationMinutes * 60 * 1000)
}

// Get retrieves cached weather telemetry if present and not expired.
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

// Put stores weather telemetry into cache associated with the target query.
func (cs *CacheService) Put(city string, response WeatherResponse) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := strings.ToLower(city)
	cs.cache[key] = response
}
