package middleware

import (
	"net/http"
	"strconv"

	"goproxy/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type RateLimiter interface {
	RateLimitMiddleware() gin.HandlerFunc
}

type rateLimiter struct {
	config        *config.Config
	logger        *zap.Logger
	globalLimiter *rate.Limiter
}

func NewRateLimiter(config *config.Config) RateLimiter {
	var perRequestLimit int
	var err error

	if perRequestLimit, err = strconv.Atoi(config.PerRequestLimit); err != nil {
		perRequestLimit = 50
	}

	globalLimiter := rate.NewLimiter(rate.Limit(float64(perRequestLimit)/60.0), perRequestLimit) // n requests per minute

	return &rateLimiter{
		config:        config,
		logger:        config.Logger,
		globalLimiter: globalLimiter,
	}
}

func (r *rateLimiter) RateLimitMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {

		if !r.globalLimiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": "You have exceeded the request limit."})
			return
		}

		c.Next()
	}
}
