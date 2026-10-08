package middleware

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/umohsamuel/elcompresso/pkg/response"
)

// RateLimiter is a per-client-IP token bucket.
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	perSecond float64
	burst     float64
	lastSweep time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(perMinute int) *RateLimiter {
	return &RateLimiter{
		buckets:   map[string]*bucket{},
		perSecond: float64(perMinute) / 60,
		burst:     float64(perMinute),
		lastSweep: time.Now(),
	}
}

// allow reports whether the key may proceed, and if not, how long to wait.
func (l *RateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()

	if now.Sub(l.lastSweep) > 5*time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.last) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.perSecond)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	return false, time.Duration((1 - b.tokens) / l.perSecond * float64(time.Second))
}

func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if l.perSecond <= 0 {
			c.Next()
			return
		}
		ok, wait := l.allow(c.ClientIP())
		if !ok {
			c.Header("Retry-After", fmt.Sprintf("%.0f", math.Ceil(wait.Seconds())))
			response.NewErrorResponse(fmt.Errorf("too many requests, try again in %.0fs", math.Ceil(wait.Seconds())), http.StatusTooManyRequests).Send(c)
			return
		}
		c.Next()
	}
}
