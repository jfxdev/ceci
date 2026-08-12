package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"ceci/backend/internal/dto"
)

// RateLimitPerIP throttles requests per client IP using a token bucket
// (burst tokens refilled at the given rate), evicting buckets idle for
// longer than 10 minutes so memory doesn't grow unbounded. Intended for
// auth endpoints (login) to slow down credential-stuffing/brute-force.
func RateLimitPerIP(requestsPerMinute int, burst int) gin.HandlerFunc {
	type entry struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var mu sync.Mutex
	buckets := make(map[string]*entry)
	limit := rate.Limit(float64(requestsPerMinute) / 60.0)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		e, ok := buckets[ip]
		if !ok {
			e = &entry{limiter: rate.NewLimiter(limit, burst)}
			buckets[ip] = e
		}
		e.lastSeen = time.Now()
		allowed := e.limiter.Allow()
		for k, v := range buckets {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(buckets, k)
			}
		}
		mu.Unlock()

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{Error: "too many requests, slow down"})
			return
		}
		c.Next()
	}
}
