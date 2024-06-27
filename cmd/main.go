package main

import (
	"context"
	"flag"
	"goproxy/internal/config"
	"goproxy/internal/router"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var (
	serviceName string
	port        string
)

func main() {
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	config, err := config.GetConfig()
	if err != nil {
		log.Fatal("Fatal error on loading config", err)
	}

	logger := config.Logger

	flag.StringVar(&serviceName, "service", "", "Name of the service")
	flag.StringVar(&port, "port", config.GoServicePort, "Port of the service")

	flag.Parse()

	if serviceName == "" {
		logger.Fatal("Please provide a service name")
	}

	config.ServiceName = serviceName

	if _, err := strconv.Atoi(port); err != nil {
		logger.Warn("Invalid Rest port provided. Using default port",
			zap.String("providedPort", port),
			zap.String("defaultPort", config.GoServicePort))
	} else {
		config.GoServicePort = port
	}

	logger.Info("Starting the service...",
		zap.String("AppName", config.AppName),
		zap.String("Service", serviceName),
		zap.String("RestPort", port))

	logger.Info("---------building packages---------")

	//config.IsLeader = make(chan bool, 1)

	//ctx := context.Background()

	// logger.Info("---------leader election-----------")
	// ele, err := leader.NewLeaderElectionManager(ctx, config)
	// if err != nil {
	// 	logger.Fatal("Error initializing Etcd leader election", zap.Error(err))
	// }

	// if err := ele.SetNodeReadiness(ctx); err != nil {
	// 	logger.Warn("Failed to set node readiness", zap.Error(err))
	// }

	// etcd leader election - in case where leader election is required
	// nodeCount, err := ele.CountReadyNodes(ctx)
	// if err != nil {
	// 	logger.Warn("Failed to count ready nodes", zap.Error(err))
	// } else {
	// 	config.EtcdNodes = nodeCount
	// }

	// config.IsLeader <- false
	// go func() {
	// 	for {
	// 		isLeader := <-config.IsLeader
	// 		if !isLeader {
	// 			if err := ele.BecomeLeader(ctx); err != nil {
	// 				logger.Warn("Failed to become leader", zap.Error(err))
	// 				config.IsLeader <- false
	// 			} else {
	// 				config.EtcdNodes = 1 // no pending latency *1
	// 				config.IsLeader <- true
	// 				return
	// 			}
	// 		}

	// 		if err := ele.SetNodeReadiness(ctx); err != nil {
	// 			logger.Warn("Failed to refresh node readiness", zap.Error(err))
	// 		}

	// 		nodeCount, err := ele.CountReadyNodes(ctx)
	// 		if err != nil {
	// 			logger.Warn("Failed to count ready nodes", zap.Error(err))
	// 		} else {
	// 			if nodeCount >= 5 {
	// 				nodeCount = 5
	// 			}

	// 			config.EtcdNodes = nodeCount
	// 		}

	// 		time.Sleep(10 * time.Second)
	// 	}
	// }()

	routerController := router.NewRouterController(config)

	// routerController.GetRouter().GET("/debug/pprof/", gin.WrapF(pprof.Index))
	// routerController.GetRouter().GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	// routerController.GetRouter().GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	// routerController.GetRouter().GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	// routerController.GetRouter().POST("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	// routerController.GetRouter().GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	// routerController.GetRouter().GET("/debug/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	// routerController.GetRouter().GET("/debug/pprof/heap", gin.WrapH(pprof.Handler("heap")))
	// routerController.GetRouter().GET("/debug/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	// routerController.GetRouter().GET("/debug/pprof/block", gin.WrapH(pprof.Handler("block")))

	srv := &http.Server{
		Addr:    ":" + config.GoServicePort,
		Handler: routerController.GetRouter(),
	}

	go func() {
		logger.Info("REST Server is starting", zap.String("port", config.GoServicePort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Error running REST server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	logger.Info("Server is shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	//ele.Close()

	logger.Info("Server exiting")
}
