package app

import (
	"container/list"
	"sync"
	"time"

	"komorebi/internal/domain/environment"
)

// pointForecastCache is a small in-memory LRU of hourly point forecasts keyed
// by snapped grid cell. Each entry carries its own expiry so successful
// forecasts and remembered failures (nil rows) can live for different TTLs.
type pointForecastCache struct {
	mu       sync.Mutex
	capacity int
	now      func() time.Time
	order    *list.List // front = most recently used
	entries  map[string]*list.Element
}

type forecastEntry struct {
	key       string
	rows      []environment.WeatherGrid
	expiresAt time.Time
}

func newPointForecastCache(capacity int) *pointForecastCache {
	return &pointForecastCache{
		capacity: capacity,
		now:      time.Now,
		order:    list.New(),
		entries:  make(map[string]*list.Element),
	}
}

func (c *pointForecastCache) get(key string) ([]environment.WeatherGrid, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	entry := el.Value.(*forecastEntry)
	if !c.now().Before(entry.expiresAt) {
		c.order.Remove(el)
		delete(c.entries, key)
		return nil, false
	}
	c.order.MoveToFront(el)
	return entry.rows, true
}

// put stores rows for key until ttl elapses. Nil rows record a failed fetch.
func (c *pointForecastCache) put(key string, rows []environment.WeatherGrid, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	expiresAt := c.now().Add(ttl)
	if el, ok := c.entries[key]; ok {
		entry := el.Value.(*forecastEntry)
		entry.rows = rows
		entry.expiresAt = expiresAt
		c.order.MoveToFront(el)
		return
	}
	c.entries[key] = c.order.PushFront(&forecastEntry{key: key, rows: rows, expiresAt: expiresAt})
	for len(c.entries) > c.capacity {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.entries, oldest.Value.(*forecastEntry).key)
	}
}
