package connpool

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ritikchawla/load-balancer/internal/config"
)

// Pool manages a pool of network connections
type Pool struct {
	mu sync.Mutex

	// Configuration
	maxIdle     int
	maxActive   int
	idleTimeout time.Duration

	// Connection management
	active   int
	idle     map[string][]*idleConn
	dialFunc func(addr string) (net.Conn, error)
}

type idleConn struct {
	conn      net.Conn
	timeAdded time.Time
}

// New creates a new connection pool
func New(cfg config.PoolConfig) (*Pool, error) {
	if cfg.MaxIdle <= 0 || cfg.MaxActive <= 0 {
		return nil, fmt.Errorf("invalid pool configuration")
	}

	p := &Pool{
		maxIdle:     cfg.MaxIdle,
		maxActive:   cfg.MaxActive,
		idleTimeout: cfg.IdleTimeout,
		idle:        make(map[string][]*idleConn),
		dialFunc: func(addr string) (net.Conn, error) {
			return net.DialTimeout("tcp", addr, 5*time.Second)
		},
	}

	// Start cleanup routine
	go p.cleanup()

	return p, nil
}

// Get gets a connection from the pool
func (p *Pool) Get(addr string) (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check for idle connections
	if conns, ok := p.idle[addr]; ok && len(conns) > 0 {
		// Get last connection
		conn := conns[len(conns)-1]
		p.idle[addr] = conns[:len(conns)-1]

		// Check if connection is still valid
		if time.Since(conn.timeAdded) > p.idleTimeout {
			conn.conn.Close()
			return p.createConn(addr)
		}

		p.active++
		return conn.conn, nil
	}

	return p.createConn(addr)
}

// Put returns a connection to the pool
func (p *Pool) Put(conn net.Conn) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.active <= 0 {
		return fmt.Errorf("connection not from pool")
	}

	p.active--
	addr := conn.RemoteAddr().String()

	// If we've hit max idle, close the connection
	if len(p.idle[addr]) >= p.maxIdle {
		return conn.Close()
	}

	// Add to idle pool
	p.idle[addr] = append(p.idle[addr], &idleConn{
		conn:      conn,
		timeAdded: time.Now(),
	})

	return nil
}

// Close closes the pool and all its connections
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close all idle connections
	for addr, conns := range p.idle {
		for _, conn := range conns {
			conn.conn.Close()
		}
		delete(p.idle, addr)
	}

	return nil
}

// createConn creates a new connection if limits allow
func (p *Pool) createConn(addr string) (net.Conn, error) {
	if p.active >= p.maxActive {
		return nil, fmt.Errorf("max active connections reached")
	}

	conn, err := p.dialFunc(addr)
	if err != nil {
		return nil, fmt.Errorf("error dialing connection: %w", err)
	}

	p.active++
	return conn, nil
}

// cleanup periodically removes idle connections that have timed out
func (p *Pool) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		for addr, conns := range p.idle {
			valid := make([]*idleConn, 0, len(conns))
			for _, conn := range conns {
				if time.Since(conn.timeAdded) > p.idleTimeout {
					conn.conn.Close()
					continue
				}
				valid = append(valid, conn)
			}
			if len(valid) == 0 {
				delete(p.idle, addr)
			} else {
				p.idle[addr] = valid
			}
		}
		p.mu.Unlock()
	}
}
