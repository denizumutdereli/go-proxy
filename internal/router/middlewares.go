package router

import (
	"goproxy/internal/middleware"

	"github.com/gin-gonic/gin"
)

/* direct middlewares ------------------------------------------------------------------------------ */

func (rc *routerController) ipLimiterMiddleware() middleware.IpController {
	ipControllerMiddleware := middleware.NewIPController(rc.config)
	return ipControllerMiddleware
}

func (rc *routerController) rateLimiterMiddleware() gin.HandlerFunc {
	rateimiterMiddleware := middleware.NewRateLimiter(rc.config)
	return rateimiterMiddleware.RateLimitMiddleware()
}

/* child middlewares ------------------------------------------------------------------------------- */

func (rc *routerController) ipRangeAllowedMiddleware() gin.HandlerFunc {
	return rc.ipLimiterMiddleware().IsRangeOfIPAllowed()
}

func (rc *routerController) ipAllowedMiddleware() gin.HandlerFunc {
	return rc.ipLimiterMiddleware().IPAllowedMiddleware()
}

