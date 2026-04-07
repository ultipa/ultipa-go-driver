//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

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
