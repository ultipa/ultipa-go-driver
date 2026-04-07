package unit

import (
	"testing"

	gqldb "github.com/ultipa/ultipa-go-driver/v6"
)

func TestCacheTypeValues(t *testing.T) {
	if gqldb.CacheTypeAll != 0 {
		t.Errorf("CacheTypeAll = %d, want 0", gqldb.CacheTypeAll)
	}
	if gqldb.CacheTypeAST != 1 {
		t.Errorf("CacheTypeAST = %d, want 1", gqldb.CacheTypeAST)
	}
	if gqldb.CacheTypePlan != 2 {
		t.Errorf("CacheTypePlan = %d, want 2", gqldb.CacheTypePlan)
	}
}

func TestCompactResultFields(t *testing.T) {
	r := gqldb.CompactResult{Success: true, Message: "ok"}
	if !r.Success {
		t.Error("expected Success to be true")
	}
	if r.Message != "ok" {
		t.Errorf("Message = %q, want %q", r.Message, "ok")
	}
}

func TestComputeTopologyResultFields(t *testing.T) {
	r := gqldb.ComputeTopologyResult{Ready: true, Message: "topology ready"}
	if !r.Ready {
		t.Error("expected Ready to be true")
	}
	if r.Message != "topology ready" {
		t.Errorf("Message = %q, want %q", r.Message, "topology ready")
	}
}

func TestSystemMetricsStructure(t *testing.T) {
	m := gqldb.SystemMetrics{
		Cpu: &gqldb.CpuMetrics{
			ProcessPercent: 12.5,
			SystemPercent:  45.0,
			NumCores:       8,
		},
		Memory: &gqldb.MemoryMetrics{
			ProcessRss:        1024 * 1024 * 100,
			HeapAlloc:         1024 * 1024 * 50,
			HeapSys:           1024 * 1024 * 200,
			StackInUse:        1024 * 1024,
			SystemTotal:       1024 * 1024 * 1024 * 16,
			SystemAvailable:   1024 * 1024 * 1024 * 8,
			SystemUsed:        1024 * 1024 * 1024 * 8,
			SystemUsedPercent: 50.0,
		},
		DiskIO:  nil,
		Storage: nil,
		Network: nil,
	}

	if m.Cpu == nil {
		t.Fatal("Cpu should not be nil")
	}
	if m.Cpu.NumCores != 8 {
		t.Errorf("NumCores = %d, want 8", m.Cpu.NumCores)
	}
	if m.Memory == nil {
		t.Fatal("Memory should not be nil")
	}
	if m.Memory.SystemUsedPercent != 50.0 {
		t.Errorf("SystemUsedPercent = %f, want 50.0", m.Memory.SystemUsedPercent)
	}
	if m.DiskIO != nil {
		t.Error("DiskIO should be nil")
	}
	if m.Storage != nil {
		t.Error("Storage should be nil")
	}
	if m.Network != nil {
		t.Error("Network should be nil")
	}
}

func TestSystemMetricsAllFields(t *testing.T) {
	m := gqldb.SystemMetrics{
		Cpu:     &gqldb.CpuMetrics{ProcessPercent: 10, SystemPercent: 20, NumCores: 4},
		Memory:  &gqldb.MemoryMetrics{ProcessRss: 100, HeapAlloc: 50, HeapSys: 200, StackInUse: 10, SystemTotal: 16000, SystemAvailable: 8000, SystemUsed: 8000, SystemUsedPercent: 50},
		DiskIO:  &gqldb.DiskIOMetrics{ReadBytes: 1000, WriteBytes: 2000, ReadCount: 10, WriteCount: 20},
		Storage: &gqldb.StorageMetrics{DbPath: "/data", DbSizeBytes: 5000, VolumeTotal: 100000, VolumeFree: 50000, VolumeUsed: 50000},
		Network: &gqldb.NetworkMetrics{BytesSent: 3000, BytesRecv: 4000, PacketsSent: 30, PacketsRecv: 40},
	}

	if m.DiskIO.ReadBytes != 1000 {
		t.Errorf("DiskIO.ReadBytes = %d, want 1000", m.DiskIO.ReadBytes)
	}
	if m.Storage.DbPath != "/data" {
		t.Errorf("Storage.DbPath = %q, want /data", m.Storage.DbPath)
	}
	if m.Network.BytesSent != 3000 {
		t.Errorf("Network.BytesSent = %d, want 3000", m.Network.BytesSent)
	}
}
