package gateway

import (
	"math"
	"sync"
	"time"
)

type tokenBucket struct {
	tokens   float64
	updated  time.Time
	lastSeen time.Time
}

// keyedLimiter is intentionally bounded. Keys come from socket addresses or
// verified subjects, never from arbitrary client headers.
type keyedLimiter struct {
	mu         sync.Mutex
	buckets    map[string]tokenBucket
	rate       float64
	burst      float64
	maxEntries int
}

func newKeyedLimiter(ratePerSecond float64, burst, maxEntries int) *keyedLimiter {
	return &keyedLimiter{buckets: make(map[string]tokenBucket), rate: ratePerSecond, burst: float64(burst), maxEntries: maxEntries}
}

func (l *keyedLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if bucket, ok := l.buckets[key]; ok {
		bucket.tokens = math.Min(l.burst, bucket.tokens+now.Sub(bucket.updated).Seconds()*l.rate)
		bucket.updated = now
		bucket.lastSeen = now
		if bucket.tokens < 1 {
			l.buckets[key] = bucket
			return false
		}
		bucket.tokens--
		l.buckets[key] = bucket
		return true
	}
	if len(l.buckets) >= l.maxEntries {
		l.evictOldest()
	}
	l.buckets[key] = tokenBucket{tokens: l.burst - 1, updated: now, lastSeen: now}
	return true
}

func (l *keyedLimiter) evictOldest() {
	var oldestKey string
	var oldest time.Time
	for key, bucket := range l.buckets {
		if oldestKey == "" || bucket.lastSeen.Before(oldest) {
			oldestKey, oldest = key, bucket.lastSeen
		}
	}
	delete(l.buckets, oldestKey)
}
