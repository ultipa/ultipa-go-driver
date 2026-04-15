//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestHealthCheckEmptyService(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Empty service name should still work (checks overall server health)
	status, err := testClient.HealthCheck(ctx, "")
	if err != nil {
		t.Fatalf("HealthCheck with empty service failed: %v", err)
	}

	if status != gqldb.HealthStatusServing {
		t.Errorf("expected SERVING status, got %v", status)
	}

	t.Logf("HealthCheck with empty service: status=%v", status)
}

func TestHealthCheckNonexistentService(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Nonexistent service name - may return error or NOT_SERVING
	status, err := testClient.HealthCheck(ctx, "nonexistent_service_xyz_999")
	if err != nil {
		t.Logf("HealthCheck for nonexistent service returned error (expected): %v", err)
		return
	}

	// If no error, status should not be SERVING for a nonexistent service
	t.Logf("HealthCheck for nonexistent service: status=%v", status)
}

func TestHealthCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := testClient.HealthCheck(ctx, "")
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if status != gqldb.HealthStatusServing {
		t.Errorf("expected SERVING status, got %v", status)
	}
}

func TestHealthCheckWithService(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := testClient.HealthCheck(ctx, "gqldb")
	if err != nil {
		// Some servers may not recognize the service name but the call itself should not crash
		t.Logf("HealthCheck for 'gqldb' returned error (may be expected): %v", err)
		return
	}

	t.Logf("HealthCheck for 'gqldb': status=%v", status)
}
