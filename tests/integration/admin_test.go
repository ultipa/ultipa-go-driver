//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestWarmupParser(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.WarmupParser(ctx, 2)
	if err != nil {
		t.Fatalf("WarmupParser failed: %v", err)
	}
	t.Log("WarmupParser succeeded")
}

func TestGetCacheStats(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := testClient.GetCacheStats(ctx, gqldb.CacheTypeAll)
	if err != nil {
		t.Fatalf("GetCacheStats failed: %v", err)
	}
	t.Logf("CacheStats: AST=%+v, Plan=%+v", stats.ASTStats, stats.PlanStats)
}

func TestClearCache(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.ClearCache(ctx, gqldb.CacheTypeAll)
	if err != nil {
		t.Fatalf("ClearCache failed: %v", err)
	}
	t.Log("ClearCache succeeded")
}

func TestGetStatistics(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := testClient.GetStatistics(ctx, "miniCircle")
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}
	t.Logf("Statistics: nodes=%d, edges=%d", stats.NodeCount, stats.EdgeCount)
}

func TestInvalidatePermissionCache(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := testClient.InvalidatePermissionCache(ctx, "")
	if err != nil {
		t.Fatalf("InvalidatePermissionCache failed: %v", err)
	}
	t.Log("InvalidatePermissionCache succeeded")
}

func TestCompact(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := testClient.Compact(ctx)
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}
	t.Logf("Compact: success=%v, message=%s", result.Success, result.Message)
}

func TestWaitForComputeTopology(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := testClient.WaitForComputeTopology(ctx, "miniCircle", 5*time.Second)
	if err != nil {
		t.Logf("WaitForComputeTopology returned error (may not be supported): %v", err)
		return
	}
	t.Logf("WaitForComputeTopology: ready=%v, message=%s", result.Ready, result.Message)
}

func TestGetSystemMetrics(t *testing.T) {
	if testClient == nil {
		t.Skip("Auth client not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metrics, err := testClient.GetSystemMetrics(ctx)
	if err != nil {
		t.Fatalf("GetSystemMetrics failed: %v", err)
	}

	if metrics.Cpu != nil {
		t.Logf("CPU: process=%.2f%%, system=%.2f%%, cores=%d",
			metrics.Cpu.ProcessPercent, metrics.Cpu.SystemPercent, metrics.Cpu.NumCores)
	}
	if metrics.Memory != nil {
		t.Logf("Memory: rss=%d, heapAlloc=%d, systemTotal=%d, systemUsed=%.2f%%",
			metrics.Memory.ProcessRss, metrics.Memory.HeapAlloc,
			metrics.Memory.SystemTotal, metrics.Memory.SystemUsedPercent)
	}
	if metrics.DiskIO != nil {
		t.Logf("DiskIO: readBytes=%d, writeBytes=%d",
			metrics.DiskIO.ReadBytes, metrics.DiskIO.WriteBytes)
	}
	if metrics.Storage != nil {
		t.Logf("Storage: path=%s, dbSize=%d, volumeTotal=%d",
			metrics.Storage.DbPath, metrics.Storage.DbSizeBytes, metrics.Storage.VolumeTotal)
	}
	if metrics.Network != nil {
		t.Logf("Network: sent=%d, recv=%d",
			metrics.Network.BytesSent, metrics.Network.BytesRecv)
	}
}
