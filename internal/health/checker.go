package health

import (
	"context"
	"math"
	"net"
	"sync"
	"time"
)

const (
	sampleSize = 1000
	// phiThreshold is the minimum value for considering a node as failed
	defaultPhiThreshold = 8.0
)

// HealthUpdateFunc is called when a backend's health status changes
type HealthUpdateFunc func(host string, healthy bool)

// Checker implements phi-accrual failure detection
type Checker struct {
	mu sync.RWMutex

	// Configuration
	interval     time.Duration
	phiThreshold float64

	// State tracking
	histories map[string]*history
	lastCheck map[string]time.Time
}

// history tracks the health check timing history for a backend
type history struct {
	mu     sync.RWMutex
	times  []time.Duration
	index  int
	count  int
	mean   time.Duration
	stdDev time.Duration
}

// New creates a new health checker
func New(interval time.Duration, phiThreshold float64) *Checker {
	if phiThreshold <= 0 {
		phiThreshold = defaultPhiThreshold
	}

	return &Checker{
		interval:     interval,
		phiThreshold: phiThreshold,
		histories:    make(map[string]*history),
		lastCheck:    make(map[string]time.Time),
	}
}

// Start begins the health checking process
func (c *Checker) Start(ctx context.Context, updateFunc HealthUpdateFunc) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAll(updateFunc)
		}
	}
}

// checkAll performs health checks on all backends
func (c *Checker) checkAll(updateFunc HealthUpdateFunc) {
	c.mu.RLock()
	hosts := make([]string, 0, len(c.histories))
	for host := range c.histories {
		hosts = append(hosts, host)
	}
	c.mu.RUnlock()

	for _, host := range hosts {
		go func(host string) {
			healthy := c.check(host)
			updateFunc(host, healthy)
		}(host)
	}
}

// check performs a health check on a single backend
func (c *Checker) check(host string) bool {
	start := time.Now()

	// Attempt connection
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		c.recordFailure(host)
		return false
	}
	conn.Close()

	// Record successful check
	c.recordSuccess(host, time.Since(start))
	return true
}

// recordSuccess updates timing history for successful health checks
func (c *Checker) recordSuccess(host string, duration time.Duration) {
	c.mu.Lock()
	if _, exists := c.histories[host]; !exists {
		c.histories[host] = &history{
			times: make([]time.Duration, sampleSize),
		}
	}
	c.lastCheck[host] = time.Now()
	c.mu.Unlock()

	hist := c.histories[host]
	hist.mu.Lock()
	defer hist.mu.Unlock()

	hist.times[hist.index] = duration
	hist.index = (hist.index + 1) % sampleSize
	if hist.count < sampleSize {
		hist.count++
	}

	// Update statistics
	hist.updateStats()
}

// recordFailure records a failed health check
func (c *Checker) recordFailure(host string) {
	c.mu.Lock()
	c.lastCheck[host] = time.Now()
	c.mu.Unlock()
}

// updateStats recalculates mean and standard deviation
func (h *history) updateStats() {
	if h.count == 0 {
		return
	}

	var sum time.Duration
	for i := 0; i < h.count; i++ {
		sum += h.times[i]
	}
	h.mean = sum / time.Duration(h.count)

	var sumSquares float64
	for i := 0; i < h.count; i++ {
		diff := float64(h.times[i] - h.mean)
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(h.count)
	h.stdDev = time.Duration(math.Sqrt(variance))
}

// phi calculates the phi value for failure detection
func (c *Checker) phi(host string) float64 {
	c.mu.RLock()
	lastTime, ok := c.lastCheck[host]
	if !ok {
		c.mu.RUnlock()
		return 0.0
	}
	c.mu.RUnlock()

	hist := c.histories[host]
	if hist == nil {
		return 0.0
	}

	hist.mu.RLock()
	defer hist.mu.RUnlock()

	if hist.count == 0 {
		return 0.0
	}

	timeSinceLastCheck := time.Since(lastTime)
	stdDev := float64(hist.stdDev)
	mean := float64(hist.mean)

	if stdDev == 0 {
		stdDev = float64(hist.mean) / 10
	}

	y := (float64(timeSinceLastCheck) - mean) / stdDev
	return -math.Log10(normalCDF(-y))
}

// normalCDF calculates the cumulative distribution function for a normal distribution
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// IsHealthy returns whether a backend is considered healthy
func (c *Checker) IsHealthy(host string) bool {
	return c.phi(host) < c.phiThreshold
}
