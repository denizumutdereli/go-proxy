package router

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"crypto/tls"

	"github.com/RackSec/srslog"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

func (rc *routerController) GetRouter() *gin.Engine {
	return rc.router
}

func (rc *routerController) GetRoutes() []RouteInfo {
	return rc.routes
}

func (rc *routerController) registerRoutes() {
	var routes []RouteInfo
	for _, route := range rc.router.Routes() {
		routes = append(routes, RouteInfo{
			Method:  route.Method,
			Path:    route.Path,
			Handler: route.Handler,
		})
	}
	rc.routes = routes
}

func (rc *routerController) newRouteGroup(groupPrefix, groupName string, middlewares ...gin.HandlerFunc) *gin.RouterGroup {
	var group *gin.RouterGroup
	if existingGroup, exists := rc.groups[groupPrefix]; exists {
		group = existingGroup
	} else {
		group = rc.router.Group(groupPrefix)

		for _, middleware := range middlewares {
			group.Use(middleware)
		}
		rc.groups[groupPrefix] = group
		rc.groupNames[groupName] = groupPrefix
	}
	return group
}

func (rc *routerController) registerGroup(group *gin.RouterGroup, parentGroup *gin.RouterGroup) {
	if parentGroup != nil {
		nestedGroup := parentGroup.Group(group.BasePath())
		*group = *nestedGroup
	} else {
		rc.router.Group(group.BasePath())
	}
}

func (rc *routerController) getGroup(groupName string) *gin.RouterGroup {
	if groupPrefix, exists := rc.groupNames[groupName]; exists {
		return rc.groups[groupPrefix]
	}
	rc.logger.Error("group name not found", zap.String("group", groupName))
	return nil
}

func (rc *routerController) registerRoutesToGroup(group *gin.RouterGroup, routes []RouteDefinition) {
	methodToHandler := map[string]func(string, ...gin.HandlerFunc) gin.IRoutes{
		http.MethodGet:     group.GET,
		http.MethodPost:    group.POST,
		http.MethodPut:     group.PUT,
		http.MethodDelete:  group.DELETE,
		http.MethodOptions: group.OPTIONS,
	}

	for _, route := range routes {
		handlers := append(route.Middlewares, route.HandlerFunc)
		if handlerFunc, ok := methodToHandler[route.Method]; ok {
			handlerFunc(route.Path, handlers...)
		} else {
			rc.logger.Error("HTTP method not supported", zap.String("method", route.Method))
		}
	}
}

func (rc *routerController) handleProxyRequest(c *gin.Context) {
	httpError := tollbooth.LimitByKeys(rc.rateLimiter, []string{c.ClientIP()})
	if httpError != nil {
		c.JSON(httpError.StatusCode, gin.H{"message": httpError.Message})
		return
	}

	r := c.Request
	w := c.Writer

	r.URL.Path = path.Clean(r.URL.Path)

	if strings.HasPrefix(r.URL.Path, "/api/") { // normalization
		proxyPath := path.Join("/", path.Base(r.URL.Path))
		r.URL.Path = proxyPath
	}

	proxy := rc.getProxy(r)

	appName := rc.config.AppName
	useSyslog := rc.config.SysLog == "true"

	var syslogger *srslog.Writer
	if useSyslog {
		var err error
		syslogger, err = srslog.Dial("", "", srslog.LOG_INFO, "CEF0")
		if err != nil {
			rc.logger.Fatal("Error setting up syslog", zap.Error(err))
		}
	}

	log := fmt.Sprintf("App: %s and proxy url: %s%s", appName, r.Host, r.URL.Path)
	if useSyslog {
		syslogger.Info(log)
	} else {
		fmt.Println(log)
	}

	proxy.ServeHTTP(w, r)
}

func (rc *routerController) getProxy(r *http.Request) *httputil.ReverseProxy {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	requestIdentifier := fmt.Sprintf("%s-%s", r.URL.Path, r.URL.RawQuery)
	cachedURL, found := rc.requestCache.Get(requestIdentifier)
	if found {
		return rc.proxies[cachedURL.(string)]
	}

	selectedURL := rc.urls[rand.Intn(len(rc.urls))]
	proxy := rc.proxies[selectedURL]
	rc.requestCache.Set(requestIdentifier, selectedURL, cache.DefaultExpiration)
	return proxy
}

func (rc *routerController) initializeProxies() {
	subServices := rc.config.SubServices
	if len(subServices) == 0 {
		rc.logger.Fatal("Error: SUB_SERVICES not set")
	}

	rc.urls = subServices

	for _, urlString := range rc.urls {
		url, err := url.Parse(urlString)
		if err != nil {
			rc.logger.Fatal("Invalid URL: %s", zap.String("url", urlString), zap.Error(err))
		}
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		rc.proxies[urlString] = proxy
	}

	cacheExpiry, err := strconv.Atoi(rc.config.CacheExpiry)
	if err != nil {
		rc.logger.Fatal("Error parsing CACHE_EXPIRY", zap.Error(err))
	}
	rc.requestCache = cache.New(time.Duration(cacheExpiry)*time.Second, 10*time.Minute)

	perRequestLimit, err := strconv.ParseFloat(rc.config.PerRequestLimit, 64)
	if err != nil {
		rc.logger.Fatal("Error parsing PER_REQUEST_LIMIT", zap.Error(err))
	}

	rc.rateLimiter = tollbooth.NewLimiter(perRequestLimit, &limiter.ExpirableOptions{
		DefaultExpirationTTL: 0,
		ExpireJobInterval:    0,
	})
	rc.rateLimiter.SetMessage("You have reached the request limit.")
}
