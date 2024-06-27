package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RackSec/srslog"
	"github.com/didip/tollbooth/v6"
	"github.com/didip/tollbooth/v6/limiter"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/patrickmn/go-cache"
)

type JsonResponse struct {
	Message string `json:"message"`
}

var requestCache *cache.Cache
var proxies map[string]*httputil.ReverseProxy
var mu sync.Mutex
var urls []string

func handleApiRequest(w http.ResponseWriter, r *http.Request) {
	response := JsonResponse{Message: "Hello from GoProxy API!"}
	json.NewEncoder(w).Encode(response)
}

func getProxy(r *http.Request) *httputil.ReverseProxy {
	mu.Lock()
	defer mu.Unlock()

	requestIdentifier := fmt.Sprintf("%s-%s", r.URL.Path, r.URL.RawQuery)
	cachedURL, found := requestCache.Get(requestIdentifier)
	if found {
		return proxies[cachedURL.(string)]
	}

	selectedURL := urls[rand.Intn(len(urls))]
	proxy := proxies[selectedURL]
	requestCache.Set(requestIdentifier, selectedURL, cache.DefaultExpiration)
	return proxy
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file:", err)
		os.Exit(1)
	}

	subServices := os.Getenv("SUB_SERVICES")
	if subServices == "" {
		log.Println("Error: SUB_SERVICES not set")
		os.Exit(1)
	}

	urls = strings.Split(subServices, ",")

	proxies = make(map[string]*httputil.ReverseProxy)
	for _, urlString := range urls {
		url, err := url.Parse(urlString)
		if err != nil {
			log.Fatalf("Invalid URL: %s", urlString)
			os.Exit(1)
		}
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		proxies[urlString] = proxy
	}

	cacheExpiry, err := strconv.Atoi(os.Getenv("CACHE_EXPIRY"))
	if err != nil {
		log.Println("Error parsing CACHE_EXPIRY:", err)
		os.Exit(1)
	}
	requestCache = cache.New(time.Duration(cacheExpiry)*time.Second, 10*time.Minute)

	appName := os.Getenv("APP_NAME")
	goServicePort := os.Getenv("GO_SERVICE_PORT")
	useSyslog := os.Getenv("SYSLOG") == "true"

	perRequestLimit, err := strconv.ParseFloat(os.Getenv("PER_REQUEST_LIMIT"), 64)
	if err != nil {
		log.Println("Error parsing PER_REQUEST_LIMIT:", err)
		os.Exit(1)
	}

	// Create a rate limiter
	rateLimiter := tollbooth.NewLimiter(perRequestLimit, &limiter.ExpirableOptions{
		DefaultExpirationTTL: 0,
		ExpireJobInterval:    0,
	})
	rateLimiter.SetMessage("You have reached the request limit.")

	router := mux.NewRouter()

	router.Handle("/", tollbooth.LimitFuncHandler(rateLimiter, handleApiRequest)).Methods("GET")

	var syslogger *srslog.Writer
	if useSyslog {
		syslogger, err = srslog.Dial("", "", srslog.LOG_INFO, "CEF0")
		if err != nil {
			log.Println("Error setting up syslog:", err)
			os.Exit(1)
		}
	}

	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpError := tollbooth.LimitByRequest(rateLimiter, w, r)
		if httpError != nil {
			w.Header().Add("Content-Type", rateLimiter.GetMessageContentType())
			w.WriteHeader(httpError.StatusCode)
			w.Write([]byte(httpError.Message))
			return
		}

		r.URL.Path = path.Clean(r.URL.Path)

		proxy := getProxy(r)

		log := fmt.Sprintf("App: %s and proxy url: %s%s", appName, r.Host, r.URL.Path)
		if useSyslog {
			syslogger.Info(log)
		} else {
			fmt.Println(log)
		}

		proxy.ServeHTTP(w, r)
	})

	certFolder := os.Getenv("CERT_FOLDER")
	if certFolder == "" {
		log.Println("CERT_FOLDER not set in environment variables")
		os.Exit(1)
	}

	certFile := path.Join(certFolder, "cert.pem")
	keyFile := path.Join(certFolder, "key.pem")

	https := os.Getenv("HTTPS") == "true"

	rand.Seed(time.Now().UnixNano())

	if https {
		log.Printf("Go service listening on HTTPS %s", goServicePort)
		err = http.ListenAndServeTLS(":"+goServicePort, certFile, keyFile, router)
	} else {
		log.Printf("Go service listening on HTTP %s", goServicePort)
		err = http.ListenAndServe(":"+goServicePort, router)
	}

	if err != nil {
		log.Println("Error starting the Go service:", err)
		os.Exit(1)
	}
}
