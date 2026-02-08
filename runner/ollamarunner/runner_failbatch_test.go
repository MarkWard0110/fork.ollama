package ollamarunner

import (
	"context"
	"testing"

	"golang.org/x/sync/semaphore"

	"github.com/ollama/ollama/kvcache"
	"github.com/ollama/ollama/llm"
	"github.com/ollama/ollama/ml"
)

func TestFailBatchPrefersKvGrowCellsMessage(t *testing.T) {
	s := &Server{parallel: 1}
	s.seqs = make([]*Sequence, 1)
	s.seqsSem = semaphore.NewWeighted(1)
	if err := s.seqsSem.Acquire(context.Background(), 1); err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	seq := &Sequence{
		responses: make(chan response, 1),
		embedding: make(chan []float32, 1),
		quit:      make(chan bool, 1),
		cache:     &InputCacheSlot{},
	}
	s.seqs[0] = seq

	b := batchState{seqs: []*Sequence{seq}}
	err := kvcache.ErrKvCacheGrow{FromCells: 4096, ToCells: 5120, MaxCells: 262144, Err: ml.ErrNoMem{}}

	s.failBatch(b, err)

	if seq.doneReason != llm.DoneReasonError {
		t.Fatalf("expected doneReason %v, got %v", llm.DoneReasonError, seq.doneReason)
	}
	want := "insufficient memory while expanding KV cache (cells 4096 -> 5120, max 262144)"
	if seq.errMsg != want {
		t.Fatalf("expected errMsg %q, got %q", want, seq.errMsg)
	}
}
