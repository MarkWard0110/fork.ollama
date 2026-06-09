package server

import (
	"strings"
	"testing"

	"github.com/ollama/ollama/llm"
	"github.com/ollama/ollama/types/model"
)

func TestModelOptionsNumCtxPriority(t *testing.T) {
	tests := []struct {
		name           string
		envContextLen  string // empty means not set (uses 0 sentinel)
		defaultNumCtx  int    // VRAM-based default
		modelNumCtx    int    // 0 means not set in model
		requestNumCtx  int    // 0 means not set in request
		expectedNumCtx int
	}{
		{
			name:           "vram default when nothing else set",
			envContextLen:  "",
			defaultNumCtx:  32768,
			modelNumCtx:    0,
			requestNumCtx:  0,
			expectedNumCtx: 32768,
		},
		{
			name:           "env var overrides vram default",
			envContextLen:  "8192",
			defaultNumCtx:  32768,
			modelNumCtx:    0,
			requestNumCtx:  0,
			expectedNumCtx: 8192,
		},
		{
			name:           "model overrides vram default",
			envContextLen:  "",
			defaultNumCtx:  32768,
			modelNumCtx:    16384,
			requestNumCtx:  0,
			expectedNumCtx: 16384,
		},
		{
			name:           "model overrides env var",
			envContextLen:  "8192",
			defaultNumCtx:  32768,
			modelNumCtx:    16384,
			requestNumCtx:  0,
			expectedNumCtx: 16384,
		},
		{
			name:           "request overrides everything",
			envContextLen:  "8192",
			defaultNumCtx:  32768,
			modelNumCtx:    16384,
			requestNumCtx:  4096,
			expectedNumCtx: 4096,
		},
		{
			name:           "request overrides vram default",
			envContextLen:  "",
			defaultNumCtx:  32768,
			modelNumCtx:    0,
			requestNumCtx:  4096,
			expectedNumCtx: 4096,
		},
		{
			name:           "request overrides model",
			envContextLen:  "",
			defaultNumCtx:  32768,
			modelNumCtx:    16384,
			requestNumCtx:  4096,
			expectedNumCtx: 4096,
		},
		{
			name:           "low vram tier default",
			envContextLen:  "",
			defaultNumCtx:  4096,
			modelNumCtx:    0,
			requestNumCtx:  0,
			expectedNumCtx: 4096,
		},
		{
			name:           "high vram tier default",
			envContextLen:  "",
			defaultNumCtx:  262144,
			modelNumCtx:    0,
			requestNumCtx:  0,
			expectedNumCtx: 262144,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set or clear environment variable
			if tt.envContextLen != "" {
				t.Setenv("OLLAMA_CONTEXT_LENGTH", tt.envContextLen)
			}

			// Create server with VRAM-based default
			s := &Server{
				defaultNumCtx: tt.defaultNumCtx,
			}

			// Create model options (use float64 as FromMap expects JSON-style numbers)
			var modelOpts map[string]any
			if tt.modelNumCtx != 0 {
				modelOpts = map[string]any{"num_ctx": float64(tt.modelNumCtx)}
			}
			model := &Model{
				Options: modelOpts,
			}

			// Create request options (use float64 as FromMap expects JSON-style numbers)
			var requestOpts map[string]any
			if tt.requestNumCtx != 0 {
				requestOpts = map[string]any{"num_ctx": float64(tt.requestNumCtx)}
			}

			opts, err := s.modelOptions(model, requestOpts)
			if err != nil {
				t.Fatalf("modelOptions failed: %v", err)
			}

			if opts.NumCtx != tt.expectedNumCtx {
				t.Errorf("NumCtx = %d, want %d", opts.NumCtx, tt.expectedNumCtx)
			}
		})
	}
}

func TestModelOptionsLlamaCppCtxSizeClamp(t *testing.T) {
	tests := []struct {
		name             string
		defaultNumCtx    int // VRAM-based default
		modelNumCtx      int // 0 means not set in model
		llamacppCtxSize  int // 0 means not set in model
		requestNumCtx    int // 0 means not set in request
		expectedNumCtx   int
		expectedErrorSub string // non-empty means an error is expected containing this substring
	}{
		{
			name:             "default num_ctx silently capped by llamacpp_ctx_size",
			defaultNumCtx:    32768,
			modelNumCtx:      0,
			llamacppCtxSize:  8192,
			requestNumCtx:    0,
			expectedNumCtx:   8192,
			expectedErrorSub: "",
		},
		{
			name:             "model num_ctx silently capped by llamacpp_ctx_size",
			defaultNumCtx:    32768,
			modelNumCtx:      16384,
			llamacppCtxSize:  8192,
			requestNumCtx:    0,
			expectedNumCtx:   8192,
			expectedErrorSub: "",
		},
		{
			name:             "explicit request num_ctx rejected when exceeding llamacpp_ctx_size",
			defaultNumCtx:    32768,
			modelNumCtx:      0,
			llamacppCtxSize:  8192,
			requestNumCtx:    32768,
			expectedNumCtx:   0,
			expectedErrorSub: "exceeds model's llamacpp_ctx_size",
		},
		{
			name:             "explicit request num_ctx within llamacpp_ctx_size accepted",
			defaultNumCtx:    32768,
			modelNumCtx:      0,
			llamacppCtxSize:  8192,
			requestNumCtx:    4096,
			expectedNumCtx:   4096,
			expectedErrorSub: "",
		},
		{
			name:             "no clamp when num_ctx equals llamacpp_ctx_size",
			defaultNumCtx:    32768,
			modelNumCtx:      0,
			llamacppCtxSize:  16384,
			requestNumCtx:    16384,
			expectedNumCtx:   16384,
			expectedErrorSub: "",
		},
		{
			name:             "no clamp when llamacpp_ctx_size larger than num_ctx",
			defaultNumCtx:    32768,
			modelNumCtx:      0,
			llamacppCtxSize:  65536,
			requestNumCtx:    0,
			expectedNumCtx:   32768,
			expectedErrorSub: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				defaultNumCtx: tt.defaultNumCtx,
			}

			// Build model options
			modelOpts := map[string]any{"llamacpp_ctx_size": float64(tt.llamacppCtxSize)}
			if tt.modelNumCtx != 0 {
				modelOpts["num_ctx"] = float64(tt.modelNumCtx)
			}
			m := &Model{Options: modelOpts}

			// Build request options
			var requestOpts map[string]any
			if tt.requestNumCtx != 0 {
				requestOpts = map[string]any{"num_ctx": float64(tt.requestNumCtx)}
			}

			opts, err := s.modelOptions(m, requestOpts)
			if tt.expectedErrorSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedErrorSub)
				}
				if !strings.Contains(err.Error(), tt.expectedErrorSub) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tt.expectedErrorSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("modelOptions failed: %v", err)
			}

			if opts.NumCtx != tt.expectedNumCtx {
				t.Errorf("NumCtx = %d, want %d", opts.NumCtx, tt.expectedNumCtx)
			}
		})
	}
}

func TestModelOptionsEmbeddingNumBatchDefault(t *testing.T) {
	tests := []struct {
		name             string
		defaultNumCtx    int
		capabilities     []string
		modelOpts        map[string]any
		requestOpts      map[string]any
		expectedNumBatch int
	}{
		{
			name:             "embedding model defaults to embedding batch size",
			defaultNumCtx:    40960,
			capabilities:     []string{string(model.CapabilityEmbedding)},
			expectedNumBatch: llm.DefaultEmbeddingNumBatch,
		},
		{
			name:             "embedding default is capped by context",
			defaultNumCtx:    1024,
			capabilities:     []string{string(model.CapabilityEmbedding)},
			expectedNumBatch: 1024,
		},
		{
			name:             "model num_batch overrides embedding default",
			defaultNumCtx:    40960,
			capabilities:     []string{string(model.CapabilityEmbedding)},
			modelOpts:        map[string]any{"num_batch": float64(1024)},
			expectedNumBatch: 1024,
		},
		{
			name:             "request num_batch overrides embedding default",
			defaultNumCtx:    40960,
			capabilities:     []string{string(model.CapabilityEmbedding)},
			requestOpts:      map[string]any{"num_batch": float64(4096)},
			expectedNumBatch: 4096,
		},
		{
			name:             "non embedding model keeps general default",
			defaultNumCtx:    40960,
			capabilities:     []string{string(model.CapabilityCompletion)},
			expectedNumBatch: 512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{defaultNumCtx: tt.defaultNumCtx}
			m := &Model{
				Options: tt.modelOpts,
			}
			m.Config.Capabilities = tt.capabilities

			opts, err := s.modelOptions(m, tt.requestOpts)
			if err != nil {
				t.Fatalf("modelOptions failed: %v", err)
			}

			if opts.NumBatch != tt.expectedNumBatch {
				t.Fatalf("NumBatch = %d, want %d", opts.NumBatch, tt.expectedNumBatch)
			}
		})
	}
}

func TestModelOptionsDraftNumPredictDefault(t *testing.T) {
	tests := []struct {
		name        string
		model       *Model
		requestOpts map[string]any
		want        int
	}{
		{
			name:  "separate draft model keeps default enabled",
			model: &Model{DraftPath: "draft.gguf"},
			want:  4,
		},
		{
			name:  "embedded draft requires explicit parameter",
			model: &Model{},
			want:  0,
		},
		{
			name:  "model parameter enables embedded draft",
			model: &Model{Options: map[string]any{"draft_num_predict": float64(4)}},
			want:  4,
		},
		{
			name:        "request parameter enables embedded draft",
			model:       &Model{},
			requestOpts: map[string]any{"draft_num_predict": float64(8)},
			want:        8,
		},
		{
			name:        "request can disable separate draft model",
			model:       &Model{DraftPath: "draft.gguf"},
			requestOpts: map[string]any{"draft_num_predict": float64(0)},
			want:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := (&Server{}).modelOptions(tt.model, tt.requestOpts)
			if err != nil {
				t.Fatal(err)
			}
			if opts.DraftNumPredict != tt.want {
				t.Fatalf("DraftNumPredict = %d, want %d", opts.DraftNumPredict, tt.want)
			}
		})
	}
}

func TestUsesAutomaticNumBatch(t *testing.T) {
	tests := []struct {
		name        string
		modelOpts   map[string]any
		requestOpts map[string]any
		want        bool
	}{
		{
			name: "default is automatic",
			want: true,
		},
		{
			name:        "model num_batch is explicit",
			modelOpts:   map[string]any{"num_batch": float64(1024)},
			requestOpts: nil,
			want:        false,
		},
		{
			name:        "request num_batch is explicit",
			requestOpts: map[string]any{"num_batch": float64(2048)},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usesAutomaticNumBatch(&Model{Options: tt.modelOpts}, tt.requestOpts); got != tt.want {
				t.Fatalf("usesAutomaticNumBatch = %v, want %v", got, tt.want)
			}
		})
	}
}
