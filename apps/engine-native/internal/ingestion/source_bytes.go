package ingestion

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
)

// WithSourceBytes owns the snapshot, readers and temporary directory until the
// consumer returns. Success also requires a final snapshot hash verification.
// Callers retain admission/deadline and domain-specific validation ownership.
func WithSourceBytes(ctx context.Context, path, prefix, id string, b Budgets, consume func([]byte) error) error {
	policy, err := b.Within(DefaultBudgets())
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	lease, err := Acquire(ctx, path, dir, id, policy)
	if err != nil {
		return err
	}
	defer lease.Close()
	reader, err := lease.Open(ctx)
	if err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(reader, policy.InputBytes+1))
	closeErr := reader.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if int64(len(raw)) > policy.InputBytes {
		return BudgetProblem("input_bytes", "input_bytes", int64(len(raw)), policy.InputBytes)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = consume(raw); err != nil {
		return err
	}
	return lease.Verify(ctx)
}

// DecodeGzip preserves original-byte digest ownership at the caller. It accepts
// concatenated gzip members under one decoded-byte budget and checks cancellation
// throughout decompression. Plain data is subject to the same decoded ceiling.
func DecodeGzip(ctx context.Context, original []byte, limit int64) ([]byte, error) {
	if limit < 1 || limit > DefaultBudgets().DecodedBytes {
		return nil, Problem("IMPORT_BUDGET_EXCEEDED", "invalid decoded budget")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(original, []byte{31, 139}) {
		if int64(len(original)) > limit {
			return nil, BudgetProblem("decoded_bytes", "decoded_bytes", int64(len(original)), limit)
		}
		return original, nil
	}
	z, err := gzip.NewReader(bytes.NewReader(original))
	if err != nil {
		return nil, err
	}
	defer z.Close()
	body, err := io.ReadAll(io.LimitReader(&contextReadCloser{ctx: ctx, ReadCloser: z}, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, BudgetProblem("decoded_bytes", "decoded_bytes", int64(len(body)), limit)
	}
	return body, ctx.Err()
}
