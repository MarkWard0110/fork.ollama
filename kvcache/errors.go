package kvcache

import (
	"fmt"
)

// ErrKvCacheGrow indicates a dynamic KV cache growth attempt failed.
//
// It wraps the underlying error (often [ml.ErrNoMem]) so callers can
// inspect the root cause with errors.As / errors.Is.
//
// NOTE: Dynamic KV growth does not change device placement; if the grow
// fails due to memory pressure, the caller may need to reduce context
// length, free VRAM, or reload with a different device configuration.
type ErrKvCacheGrow struct {
	FromCells int
	ToCells   int
	MaxCells  int
	Err       error
}

func (e ErrKvCacheGrow) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("kv cache grow failed (cells %d -> %d, max %d)", e.FromCells, e.ToCells, e.MaxCells)
	}
	return fmt.Sprintf("kv cache grow failed (cells %d -> %d, max %d): %v", e.FromCells, e.ToCells, e.MaxCells, e.Err)
}

func (e ErrKvCacheGrow) Unwrap() error {
	return e.Err
}
