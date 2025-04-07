package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Balancer BalancerConfig  `yaml:"balancer"`
	Backends []BackendConfig `yaml:"backends"`
	Pool     PoolConfig      `yaml:"pool"`
}

// BalancerConfig holds the load balancer specific configuration
type BalancerConfig struct {
	Port                int           `yaml:"port"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
	FailureThreshold    float64       `yaml:"failure_threshold"`
}

// BackendConfig represents a single backend server configuration
type BackendConfig struct {
	Host   string `yaml:"host"`
	Port   int    `yaml:"port"`
	Weight int    `yaml:"weight"`
}

// PoolConfig represents connection pool configuration
type PoolConfig struct {
	MaxIdle     int           `yaml:"max_idle"`
	MaxActive   int           `yaml:"max_active"`
	IdleTimeout time.Duration `yaml:"idle_timeout"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// validate checks if the configuration is valid
func validate(cfg *Config) error {
	if cfg.Balancer.Port <= 0 {
		return fmt.Errorf("invalid port: %d", cfg.Balancer.Port)
	}

	if cfg.Balancer.HealthCheckInterval <= 0 {
		return fmt.Errorf("invalid health check interval: %v", cfg.Balancer.HealthCheckInterval)
	}

	if cfg.Balancer.FailureThreshold <= 0 {
		return fmt.Errorf("invalid failure threshold: %v", cfg.Balancer.FailureThreshold)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured")
	}

	for i, backend := range cfg.Backends {
		if backend.Host == "" {
			return fmt.Errorf("backend %d: missing host", i)
		}
		if backend.Port <= 0 {
			return fmt.Errorf("backend %d: invalid port: %d", i, backend.Port)
		}
		if backend.Weight <= 0 {
			return fmt.Errorf("backend %d: invalid weight: %d", i, backend.Weight)
		}
	}

	if cfg.Pool.MaxIdle <= 0 {
		return fmt.Errorf("invalid max idle connections: %d", cfg.Pool.MaxIdle)
	}

	if cfg.Pool.MaxActive <= 0 {
		return fmt.Errorf("invalid max active connections: %d", cfg.Pool.MaxActive)
	}

	if cfg.Pool.IdleTimeout <= 0 {
		return fmt.Errorf("invalid idle timeout: %v", cfg.Pool.IdleTimeout)
	}

	return nil
}
