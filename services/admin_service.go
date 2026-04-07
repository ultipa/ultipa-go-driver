package services

import (
	"context"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// AdminService handles admin operations (cache, stats, warmup).
type AdminService struct {
	ctx *ServiceContext
}

// NewAdminService creates a new AdminService.
func NewAdminService(ctx *ServiceContext) *AdminService {
	return &AdminService{
		ctx: ctx,
	}
}

// CacheStats represents cache statistics (mirrors main package).
type CacheStats struct {
	ASTStats  *ASTCacheStats
	PlanStats *PlanCacheStats
}

// ASTCacheStats represents AST cache statistics.
type ASTCacheStats struct {
	Size      int64
	Hits      int64
	Misses    int64
	Evictions int64
}

// PlanCacheStats represents plan cache statistics.
type PlanCacheStats struct {
	Size      int64
	Hits      int64
	Misses    int64
	Evictions int64
}

// Statistics represents database statistics (mirrors main package).
type Statistics struct {
	GraphCount       int64
	NodeCount        int64
	EdgeCount        int64
	PropertyCount    int64
	LabelCount       int64
	LabelCounts      map[string]uint64 // Per-label node counts
	EdgeLabelCounts  map[string]uint64 // Per-label edge counts
	MemoryUsage      int64
	DiskUsage        int64
	QueryCount       int64
	AvgQueryLatency  float64
	PeakQueryLatency float64
}

// WarmupParser warms up the parser cache.
func (s *AdminService) WarmupParser(ctx context.Context, count int) error {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.WarmupParserRequest{
		Count: int32(count),
	}

	_, err := s.ctx.AdminClient.WarmupParser(ctx, req)
	return err
}

// GetCacheStats retrieves cache statistics.
func (s *AdminService) GetCacheStats(ctx context.Context, cacheType int32) (*CacheStats, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.GetCacheStatsRequest{
		CacheType: CacheTypeToProto(cacheType),
	}

	resp, err := s.ctx.AdminClient.GetCacheStats(ctx, req)
	if err != nil {
		return nil, err
	}

	stats := &CacheStats{}
	if resp.AstStats != nil {
		stats.ASTStats = &ASTCacheStats{
			Size:      int64(resp.AstStats.Entries),
			Hits:      int64(resp.AstStats.Hits),
			Misses:    int64(resp.AstStats.Misses),
			Evictions: int64(resp.AstStats.Evictions),
		}
	}
	if resp.PlanStats != nil {
		stats.PlanStats = &PlanCacheStats{
			Size:      int64(resp.PlanStats.Size),
			Hits:      int64(resp.PlanStats.Hits),
			Misses:    int64(resp.PlanStats.Misses),
			Evictions: 0, // PlanCacheStats doesn't have Evictions field in proto
		}
	}

	return stats, nil
}

// ClearCache clears the specified cache.
func (s *AdminService) ClearCache(ctx context.Context, cacheType int32) error {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.ClearCacheRequest{
		CacheType: CacheTypeToProto(cacheType),
	}

	_, err := s.ctx.AdminClient.ClearCache(ctx, req)
	return err
}

// GetStatistics retrieves database statistics.
func (s *AdminService) GetStatistics(ctx context.Context, graphName string) (*Statistics, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.GetStatisticsRequest{
		GraphName: graphName,
	}

	resp, err := s.ctx.AdminClient.GetStatistics(ctx, req)
	if err != nil {
		return nil, err
	}

	// Calculate label count from LabelCounts map
	labelCount := int64(len(resp.LabelCounts))

	return &Statistics{
		GraphCount:       0,                    // Not available in proto
		NodeCount:        int64(resp.NodeCount),
		EdgeCount:        int64(resp.EdgeCount),
		PropertyCount:    0,                    // Not available in proto
		LabelCount:       labelCount,
		LabelCounts:      resp.LabelCounts,
		EdgeLabelCounts:  resp.EdgeLabelCounts,
		MemoryUsage:      0,                    // Not available in proto
		DiskUsage:        0,                    // Not available in proto
		QueryCount:       0,                    // Not available in proto
		AvgQueryLatency:  0,                    // Not available in proto
		PeakQueryLatency: 0,                    // Not available in proto
	}, nil
}

// InvalidatePermissionCache invalidates permission cache for a user.
func (s *AdminService) InvalidatePermissionCache(ctx context.Context, username string) error {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.InvalidatePermissionCacheRequest{
		Username: username,
	}

	_, err := s.ctx.AdminClient.InvalidatePermissionCache(ctx, req)
	return err
}

// CompactResult represents the result of Compact operation.
type CompactResult struct {
	Success bool
	Message string
}

// Compact triggers manual compaction of the database storage.
func (s *AdminService) Compact(ctx context.Context) (*CompactResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.CompactRequest{}

	resp, err := s.ctx.AdminClient.Compact(ctx, req)
	if err != nil {
		return nil, err
	}

	return &CompactResult{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}

// ComputeTopologyResult represents the result of WaitForComputeTopology.
type ComputeTopologyResult struct {
	Ready   bool
	Message string
}

// WaitForComputeTopology waits for the computing engine topology to be ready.
func (s *AdminService) WaitForComputeTopology(ctx context.Context, graphName string, timeoutMs int64) (*ComputeTopologyResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.WaitForComputeTopologyRequest{
		GraphName: graphName,
		TimeoutMs: timeoutMs,
	}

	resp, err := s.ctx.AdminClient.WaitForComputeTopology(ctx, req)
	if err != nil {
		return nil, err
	}

	return &ComputeTopologyResult{
		Ready:   resp.Ready,
		Message: resp.Message,
	}, nil
}

// CpuMetrics contains CPU usage metrics.
type CpuMetrics struct {
	ProcessPercent float64
	SystemPercent  float64
	NumCores       int32
}

// MemoryMetrics contains memory usage metrics.
type MemoryMetrics struct {
	ProcessRss        uint64
	HeapAlloc         uint64
	HeapSys           uint64
	StackInUse        uint64
	SystemTotal       uint64
	SystemAvailable   uint64
	SystemUsed        uint64
	SystemUsedPercent float64
}

// DiskIOMetrics contains disk I/O metrics.
type DiskIOMetrics struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadCount  uint64
	WriteCount uint64
}

// StorageMetrics contains storage metrics.
type StorageMetrics struct {
	DbPath      string
	DbSizeBytes uint64
	VolumeTotal uint64
	VolumeFree  uint64
	VolumeUsed  uint64
}

// NetworkMetrics contains network metrics.
type NetworkMetrics struct {
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
}

// SystemMetrics contains system-level metrics.
type SystemMetrics struct {
	Cpu     *CpuMetrics
	Memory  *MemoryMetrics
	DiskIO  *DiskIOMetrics
	Storage *StorageMetrics
	Network *NetworkMetrics
}

// GetSystemMetrics returns system-level metrics (CPU, memory, disk I/O, storage, network).
func (s *AdminService) GetSystemMetrics(ctx context.Context) (*SystemMetrics, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.GetSystemMetricsRequest{}

	resp, err := s.ctx.AdminClient.GetSystemMetrics(ctx, req)
	if err != nil {
		return nil, err
	}

	result := &SystemMetrics{}
	if resp.Cpu != nil {
		result.Cpu = &CpuMetrics{
			ProcessPercent: resp.Cpu.ProcessPercent,
			SystemPercent:  resp.Cpu.SystemPercent,
			NumCores:       resp.Cpu.NumCores,
		}
	}
	if resp.Memory != nil {
		result.Memory = &MemoryMetrics{
			ProcessRss:        resp.Memory.ProcessRss,
			HeapAlloc:         resp.Memory.HeapAlloc,
			HeapSys:           resp.Memory.HeapSys,
			StackInUse:        resp.Memory.StackInUse,
			SystemTotal:       resp.Memory.SystemTotal,
			SystemAvailable:   resp.Memory.SystemAvailable,
			SystemUsed:        resp.Memory.SystemUsed,
			SystemUsedPercent: resp.Memory.SystemUsedPercent,
		}
	}
	if resp.DiskIo != nil {
		result.DiskIO = &DiskIOMetrics{
			ReadBytes:  resp.DiskIo.ReadBytes,
			WriteBytes: resp.DiskIo.WriteBytes,
			ReadCount:  resp.DiskIo.ReadCount,
			WriteCount: resp.DiskIo.WriteCount,
		}
	}
	if resp.Storage != nil {
		result.Storage = &StorageMetrics{
			DbPath:      resp.Storage.DbPath,
			DbSizeBytes: resp.Storage.DbSizeBytes,
			VolumeTotal: resp.Storage.VolumeTotal,
			VolumeFree:  resp.Storage.VolumeFree,
			VolumeUsed:  resp.Storage.VolumeUsed,
		}
	}
	if resp.Network != nil {
		result.Network = &NetworkMetrics{
			BytesSent:   resp.Network.BytesSent,
			BytesRecv:   resp.Network.BytesRecv,
			PacketsSent: resp.Network.PacketsSent,
			PacketsRecv: resp.Network.PacketsRecv,
		}
	}

	return result, nil
}
