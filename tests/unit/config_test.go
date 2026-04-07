package unit

import (
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestDefaultConfig(t *testing.T) {
	config := gqldb.DefaultConfig()

	if len(config.Hosts) != 1 || config.Hosts[0] != "localhost:9000" {
		t.Errorf("expected default host localhost:9000, got %v", config.Hosts)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", config.Timeout)
	}

	if config.MaxRecvSize != 64*1024*1024 {
		t.Errorf("expected default max recv size 64MB, got %d", config.MaxRecvSize)
	}

	if config.PoolSize != 10 {
		t.Errorf("expected default pool size 10, got %d", config.PoolSize)
	}

	if config.HealthCheckInterval != 30*time.Second {
		t.Errorf("expected default health check interval 30s, got %v", config.HealthCheckInterval)
	}

	if config.RetryCount != 3 {
		t.Errorf("expected default retry count 3, got %d", config.RetryCount)
	}

	if config.RetryDelay != 100*time.Millisecond {
		t.Errorf("expected default retry delay 100ms, got %v", config.RetryDelay)
	}
}

func TestConfigBuilder(t *testing.T) {
	config := gqldb.NewConfigBuilder().
		Hosts("host1:9000", "host2:9000").
		Username("admin").
		Password("secret").
		DefaultGraph("myGraph").
		Timeout(60 * time.Second).
		MaxRecvSize(8 * 1024 * 1024).
		PoolSize(20).
		HealthCheckInterval(1 * time.Minute).
		RetryCount(5).
		RetryDelay(200 * time.Millisecond).
		Build()

	if len(config.Hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(config.Hosts))
	}

	if config.Hosts[0] != "host1:9000" {
		t.Errorf("expected host1:9000, got %v", config.Hosts[0])
	}

	if config.Username != "admin" {
		t.Errorf("expected username admin, got %v", config.Username)
	}

	if config.Password != "secret" {
		t.Errorf("expected password secret, got %v", config.Password)
	}

	if config.DefaultGraph != "myGraph" {
		t.Errorf("expected default graph myGraph, got %v", config.DefaultGraph)
	}

	if config.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", config.Timeout)
	}

	if config.MaxRecvSize != 8*1024*1024 {
		t.Errorf("expected max recv size 8MB, got %d", config.MaxRecvSize)
	}

	if config.PoolSize != 20 {
		t.Errorf("expected pool size 20, got %d", config.PoolSize)
	}

	if config.HealthCheckInterval != 1*time.Minute {
		t.Errorf("expected health check interval 1m, got %v", config.HealthCheckInterval)
	}

	if config.RetryCount != 5 {
		t.Errorf("expected retry count 5, got %d", config.RetryCount)
	}

	if config.RetryDelay != 200*time.Millisecond {
		t.Errorf("expected retry delay 200ms, got %v", config.RetryDelay)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *gqldb.Config
		expectError bool
	}{
		{
			name:        "valid config",
			config:      gqldb.DefaultConfig(),
			expectError: false,
		},
		{
			name: "no hosts",
			config: &gqldb.Config{
				Hosts:   []string{},
				Timeout: 30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "negative timeout",
			config: &gqldb.Config{
				Hosts:   []string{"localhost:9000"},
				Timeout: -1 * time.Second,
			},
			expectError: true,
		},
		{
			name: "zero max recv size gets default",
			config: &gqldb.Config{
				Hosts:       []string{"localhost:9000"},
				Timeout:     30 * time.Second,
				MaxRecvSize: 0,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
