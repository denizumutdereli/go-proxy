package router

import (
	"goproxy/internal/config"
	"goproxy/internal/utils"
	"net/http/httputil"
	"sync"

	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

type RouteInfo struct {
	Method  string
	Path    string
	Handler string
}

type RouteDefinition struct {
	Method      string
	Path        string
	HandlerFunc gin.HandlerFunc
	Middlewares []gin.HandlerFunc
}

type RouterController interface {
	GetRouter() *gin.Engine
	GetRoutes() []RouteInfo
	newRouteGroup(groupPrefix, groupName string, middlewares ...gin.HandlerFunc) *gin.RouterGroup
	registerGroup(group *gin.RouterGroup, parentGroup *gin.RouterGroup)
	getGroup(name string) *gin.RouterGroup
	registerRoutesToGroup(group *gin.RouterGroup, routes []RouteDefinition)
	initializeProxies()
	handleProxyRequest(c *gin.Context)
}

type routerController struct {
	config       *config.Config
	logger       *zap.Logger
	router       *gin.Engine
	routes       []RouteInfo
	groups       map[string]*gin.RouterGroup
	groupNames   map[string]string
	middlewares  map[string]gin.HandlerFunc
	requestCache *cache.Cache
	proxies      map[string]*httputil.ReverseProxy
	mu           sync.Mutex
	urls         []string
	rateLimiter  *limiter.Limiter
}

func NewRouterController(cfg *config.Config) RouterController {
	rc := &routerController{
		config:      cfg,
		logger:      cfg.Logger,
		router:      gin.Default(),
		groups:      make(map[string]*gin.RouterGroup),
		groupNames:  make(map[string]string),
		middlewares: make(map[string]gin.HandlerFunc),
		proxies:     make(map[string]*httputil.ReverseProxy),
	}

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = cfg.AllowedOrigins
	corsConfig.AllowAllOrigins = cfg.AllowAllOrigins
	corsConfig.AllowMethods = cfg.AllowedRestMethods
	corsConfig.AllowHeaders = cfg.AllowedRestHeaders

	rc.router.Use(cors.New(corsConfig))

	gin.ForceConsoleColor()

	rc.router.SetTrustedProxies(nil)
	rc.router.RemoveExtraSlash = true
	rc.router.RedirectTrailingSlash = true

	rc.router.Use(
		rc.rateLimiterMiddleware(),
		rc.ipRangeAllowedMiddleware(),
		rc.ipAllowedMiddleware(),
	)

	utils.ClearScreen()
	rc.setupDefaultInterface()
	rc.initializeProxies()
	rc.registerRoutes()

	return rc
}

func (rc *routerController) setupDefaultInterface() {
	defaultRouteGroup := rc.router.Group("/")
	rc.setupDefaultRoutes(defaultRouteGroup)
}
