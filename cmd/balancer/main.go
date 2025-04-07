package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ritikchawla/load-balancer/internal/balancer"
	"github.com/ritikchawla/load-balancer/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context that will be canceled on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create and start the load balancer
	lb, err := balancer.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create load balancer: %v", err)
	}

	// Start the load balancer in a goroutine
	go func() {
		if err := lb.Start(ctx); err != nil {
			log.Printf("Load balancer error: %v", err)
			cancel()
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	log.Println("Shutting down...")

	// Trigger graceful shutdown
	cancel()
	if err := lb.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
