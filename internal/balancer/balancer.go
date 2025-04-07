package balancer

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/ritikchawla/load-balancer/internal/config"
	"github.com/ritikchawla/load-balancer/internal/connpool"
	"github.com/ritikchawla/load-balancer/internal/hashing"
	"github.com/ritikchawla/load-balancer/internal/health"
)

// LoadBalancer represents the main load balancer interface
type LoadBalancer interface {
	Start(context.Context) error
	Shutdown(context.Context) error
}

// balancer implements the LoadBalancer interface
type balancer struct {
	cfg      *config.Config
	listener net.Listener
	pool     *connpool.Pool
	hasher   *hashing.ConsistentHasher
	health   *health.Checker
	backends sync.Map // map[string]*backend
	mu       sync.RWMutex
}

// backend represents a backend server
type backend struct {
	host   string
	port   int
	weight int
	health bool
}

// New creates a new load balancer instance
func New(cfg *config.Config) (LoadBalancer, error) {
	b := &balancer{
		cfg: cfg,
	}

	// Initialize connection pool
	pool, err := connpool.New(cfg.Pool)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	b.pool = pool

	// Initialize consistent hasher
	b.hasher = hashing.New()

	// Initialize health checker
	b.health = health.New(cfg.Balancer.HealthCheckInterval, cfg.Balancer.FailureThreshold)

	// Initialize backends
	for _, bc := range cfg.Backends {
		backend := &backend{
			host:   bc.Host,
			port:   bc.Port,
			weight: bc.Weight,
			health: true,
		}
		b.backends.Store(fmt.Sprintf("%s:%d", bc.Host, bc.Port), backend)
		b.hasher.Add(backend.host, backend.weight)
	}

	return b, nil
}

// Start begins accepting connections
func (b *balancer) Start(ctx context.Context) error {
	// Create HTTP server for health checks
	srv := &http.Server{Addr: ":8080"}
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start health check server with context cancellation
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Health check server error: %v", err)
		}
	}()

	// Ensure health check server is shutdown on context cancellation
	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("Health check server shutdown error: %v", err)
		}
	}()

	// Start main load balancer
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", b.cfg.Balancer.Port))
	if err != nil {
		return fmt.Errorf("starting listener: %w", err)
	}
	b.listener = listener

	// Start health checker
	go b.health.Start(ctx, b.updateBackendHealth)

	log.Printf("Load balancer listening on :%d", b.cfg.Balancer.Port)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Printf("Error accepting connection: %v", err)
				continue
			}
			go b.handleConnection(ctx, conn)
		}
	}
}

// Shutdown gracefully shuts down the load balancer
func (b *balancer) Shutdown(ctx context.Context) error {
	if b.listener != nil {
		if err := b.listener.Close(); err != nil {
			return fmt.Errorf("closing listener: %w", err)
		}
	}

	if err := b.pool.Close(); err != nil {
		return fmt.Errorf("closing connection pool: %w", err)
	}

	return nil
}

// handleConnection processes a single client connection
func (b *balancer) handleConnection(ctx context.Context, clientConn net.Conn) {
	defer clientConn.Close()

	// Get backend using consistent hashing
	backend, err := b.getHealthyBackend(clientConn.RemoteAddr().String())
	if err != nil {
		log.Printf("Error getting backend: %v", err)
		return
	}

	// Get backend connection from pool
	backendConn, err := b.pool.Get(fmt.Sprintf("%s:%d", backend.host, backend.port))
	if err != nil {
		log.Printf("Error getting backend connection: %v", err)
		return
	}
	defer b.pool.Put(backendConn)

	// Forward traffic between client and backend
	errCh := make(chan error, 2)
	go b.proxy(clientConn, backendConn, errCh)
	go b.proxy(backendConn, clientConn, errCh)

	// Wait for either connection to close
	<-errCh
}

// proxy copies data between two connections
func (b *balancer) proxy(dst, src net.Conn, errCh chan<- error) {
	_, err := io.Copy(dst, src)
	errCh <- err
}

// getHealthyBackend returns a healthy backend server
func (b *balancer) getHealthyBackend(key string) (*backend, error) {
	host := b.hasher.Get(key)
	if host == "" {
		return nil, fmt.Errorf("no backend available")
	}

	value, ok := b.backends.Load(host)
	if !ok {
		return nil, fmt.Errorf("backend not found: %s", host)
	}

	backend := value.(*backend)
	if !backend.health {
		return nil, fmt.Errorf("backend unhealthy: %s", host)
	}

	return backend, nil
}

// updateBackendHealth updates the health status of a backend
func (b *balancer) updateBackendHealth(host string, healthy bool) {
	if value, ok := b.backends.Load(host); ok {
		backend := value.(*backend)
		backend.health = healthy
		b.backends.Store(host, backend)
	}
}
