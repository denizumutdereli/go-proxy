package middleware

import (
	"net"
	"net/http"

	"goproxy/internal/config"

	"goproxy/internal/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type IpController interface {
	IPAllowedMiddleware() gin.HandlerFunc
	IsRangeOfIPAllowed() gin.HandlerFunc
}

type ipController struct {
	config             *config.Config
	logger             *zap.Logger
	allowedRanges      []string
	specificAllowedIPs []string
}

func NewIPController(config *config.Config) IpController {

	allowedIPs := make(map[string]struct{})
	for _, ip := range config.AllowedSpecificIps {
		allowedIPs[ip] = struct{}{}
	}

	return &ipController{
		config:             config,
		logger:             config.Logger,
		allowedRanges:      config.AllowedIpRanges,
		specificAllowedIPs: config.AllowedSpecificIps,
	}
}

func (i *ipController) IPAllowedMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userVpnAddrs := i.config.AllowedSpecificIps

		userVpnAddrs = append(userVpnAddrs, "127.0.0.1")
		clientIP := utils.GetClientIP(c)

		for _, vpn := range userVpnAddrs {
			if vpn == clientIP {
				c.Next()
				return
			}
		}

		i.logger.Debug("IP address is not allowed", zap.String("clientIP", clientIP))
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Your IP address is not permitted to access this service."})
	}
}

func (i *ipController) IsRangeOfIPAllowed() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := utils.GetClientIP(c)
		parsedIP := net.ParseIP(clientIP)

		if parsedIP == nil {
			i.logger.Debug("Invalid IP address provided in checking subset of allowed ranges")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Your IP address is not permitted to access this range of service."})
			return
		}

		for _, allowedRange := range i.allowedRanges {
			_, subnet, err := net.ParseCIDR(allowedRange)
			if err != nil {
				i.logger.Error("error parsing CIDR", zap.Error(err))
				continue
			}

			if subnet.Contains(parsedIP) {
				c.Next()
				return
			}

		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Your IP address is not permitted to access this range of service."})
	}
}
