package services

import (
	"context"
	"time"

	pb "github.com/ultipa/ultipa-go-driver/v6/proto"
)

// TransactionService handles transaction operations.
type TransactionService struct {
	ctx *ServiceContext
}

// NewTransactionService creates a new TransactionService.
func NewTransactionService(ctx *ServiceContext) *TransactionService {
	return &TransactionService{
		ctx: ctx,
	}
}

// Transaction represents a database transaction (mirrors main package).
type Transaction struct {
	ID        uint64
	SessionID uint64
	GraphName string
	ReadOnly  bool
	Timeout   time.Duration
}

// TransactionInfo represents transaction metadata (mirrors main package).
type TransactionInfo struct {
	TransactionID uint64
	SessionID     uint64
	GraphName     string
	ReadOnly      bool
	CreatedAt     int64
	DurationMs    int64
	InternalTxID  string
}

// BeginTransactionResult contains the result of beginning a transaction.
type BeginTransactionResult struct {
	TransactionID uint64
	SessionID     uint64
	GraphName     string
	ReadOnly      bool
	Timeout       time.Duration
}

// BeginTransaction starts a new transaction.
func (s *TransactionService) BeginTransaction(ctx context.Context, graphName string, readOnly bool, timeout int) (*BeginTransactionResult, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.BeginRequest{
		SessionId: s.ctx.GetSessionID(),
		GraphName: graphName,
		ReadOnly:  readOnly,
		Timeout:   int32(timeout),
	}

	resp, err := s.ctx.TransactionClient.Begin(ctx, req)
	if err != nil {
		return nil, err
	}

	return &BeginTransactionResult{
		TransactionID: resp.TransactionId,
		SessionID:     s.ctx.GetSessionID(),
		GraphName:     graphName,
		ReadOnly:      readOnly,
		Timeout:       time.Duration(timeout) * time.Second,
	}, nil
}

// Commit commits a transaction.
func (s *TransactionService) Commit(ctx context.Context, transactionID uint64) (bool, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.CommitRequest{
		SessionId:     s.ctx.GetSessionID(),
		TransactionId: transactionID,
	}

	resp, err := s.ctx.TransactionClient.Commit(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

// Rollback aborts a transaction.
func (s *TransactionService) Rollback(ctx context.Context, transactionID uint64) (bool, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.RollbackRequest{
		SessionId:     s.ctx.GetSessionID(),
		TransactionId: transactionID,
	}

	resp, err := s.ctx.TransactionClient.Rollback(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

// ListTransactions returns active transactions.
func (s *TransactionService) ListTransactions(ctx context.Context) ([]*TransactionInfo, error) {
	ctx = s.ctx.WithSessionMetadata(ctx)

	req := &pb.ListTransactionsRequest{
		SessionId: s.ctx.GetSessionID(),
	}

	resp, err := s.ctx.TransactionClient.ListTransactions(ctx, req)
	if err != nil {
		return nil, err
	}

	txs := make([]*TransactionInfo, len(resp.Transactions))
	for i, t := range resp.Transactions {
		txs[i] = &TransactionInfo{
			TransactionID: t.TransactionId,
			SessionID:     t.SessionId,
			GraphName:     t.GraphName,
			ReadOnly:      t.ReadOnly,
			CreatedAt:     t.CreatedAt,
			DurationMs:    t.DurationMs,
			InternalTxID:  t.InternalTxId,
		}
	}

	return txs, nil
}
