# OLLAMA_DEBUG_LOG_REQUESTS: Response Logging Enhancement

## Overview

Enhances the existing `OLLAMA_DEBUG_LOG_REQUESTS` feature to log HTTP response bodies alongside request bodies. This provides complete visibility into both sides of the inference API conversation — what was sent to the server and what the server replied with.

### Problem Statement

The original `OLLAMA_DEBUG_LOG_REQUESTS` feature only logged request bodies and generated curl replay commands. When debugging inference issues, developers had no way to see what the server actually responded with without attaching external tools or modifying code.

### Solution

Wrap the HTTP response writer in the existing middleware to capture all response bytes, then write them to a sibling file next to the request body file. The implementation handles:

- **Non-streaming responses**: Single JSON object → `.json`
- **Ollama API streaming**: NDJSON chunks → `.jsonl`
- **OpenAI API streaming**: SSE (`data: {...}`) → stripped to valid JSONL → `.jsonl`
- **Error responses**: Error JSON → `.json`

## Environment Variable

```bash
OLLAMA_DEBUG_LOG_REQUESTS=true
```

When enabled, all inference routes log request bodies, response bodies, and curl replay commands to a temporary directory.

## Affected Routes

All routes wrapped by `withInferenceRequestLogging()`:

| Route | API | Description |
|-------|-----|-------------|
| `/api/generate` | Ollama | Text generation |
| `/api/chat` | Ollama | Chat completion |
| `/v1/chat/completions` | OpenAI | Chat completion |
| `/v1/completions` | OpenAI | Legacy completions |
| `/v1/responses` | OpenAI | Responses API |
| `/v1/messages` | Anthropic | Messages API |

## File Naming Convention

Files are written to a temp directory (e.g., `/tmp/ollama-request-logs-123456789/`) with this pattern:

```
{timestamp}_{route}_{type}.{ext}
```

### Components

| Component | Format | Example |
|-----------|--------|---------|
| timestamp | `YYYYMMDDTHHmmss.NNNNNNNNNZ-NNNNNN` | `20260525T150405.123456789Z-000001` |
| route | sanitized route path | `v1_chat_completions` |
| type | `body`, `response`, `request` | `body` |
| ext | `json`, `jsonl`, `sh` | `json` |

### Example Files for One Request

```
20260525T150405.123456789Z-000001_v1_chat_completions_body.json       # Request body
20260525T150405.123456789Z-000001_v1_chat_completions_response.jsonl  # Response (streaming)
20260525T150405.123456789Z-000001_v1_chat_completions_request.sh      # Curl replay
```

### Extension Selection Logic

```
if response Content-Type contains "ndjson" or "event-stream":
    extension = ".jsonl"
else:
    extension = ".json"
```

## Architecture

### Middleware Chain

```
Client ←→ responseCaptureWriter ←→ OpenAI ChatWriter (if /v1/*) ←→ Handler
```

The `responseCaptureWriter` wraps the outermost `c.Writer`, so it captures the final bytes sent to the client after any middleware transformations.

### Response Writer Wrapper

```go
type responseCaptureWriter struct {
    http.ResponseWriter  // delegated writer
    body    bytes.Buffer // captured bytes
    status  int          // HTTP status code
    written bool         // has any write occurred
}
```

Implements gin's full `ResponseWriter` interface:
- `http.ResponseWriter` (Header, Write, WriteHeader)
- `http.Hijacker` (Hijack)
- `http.Flusher` (Flush)
- `http.CloseNotifier` (CloseNotify)
- gin-specific: Status, Size, Written, WriteHeaderNow, WriteString, Pusher

### SSE-to-JSONL Conversion

OpenAI API routes use Server-Sent Events (SSE) format:
```
data: {"id":"chatcmpl-1","choices":[...]}

data: {"id":"chatcmpl-1","choices":[...]}

data: [DONE]

```

The `extractJSONFromSSE()` function:
1. Splits on newlines
2. Strips `data: ` prefix from each line
3. Skips `data: [DONE]` sentinel
4. Skips empty lines and non-data lines
5. Outputs clean JSONL (one JSON object per line)

## Implementation Details

### Files Modified

| File | Change |
|------|--------|
| `server/inference_request_log.go` | Added `responseCaptureWriter`, extended `middleware()` to wrap `c.Writer`, extended `log()` to write response file, added `extractJSONFromSSE()` |
| `server/routes_request_log_test.go` | Added 4 new tests: non-streaming capture, streaming capture, error capture, SSE extraction |
| `envconfig/config.go` | Updated description for `OLLAMA_DEBUG_LOG_REQUESTS` |

### Key Code Paths

#### Middleware (server/inference_request_log.go)

```go
func (l *inferenceRequestLogger) middleware(route string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Capture request body (existing)
        body, _ := io.ReadAll(c.Request.Body)
        c.Request.Body = io.NopCloser(bytes.NewReader(body))

        // 2. Wrap response writer
        captureWriter := &responseCaptureWriter{ResponseWriter: c.Writer, status: http.StatusOK}
        c.Writer = captureWriter

        // 3. Run handlers
        c.Next()

        // 4. Log both request and response
        responseContentType := c.Writer.Header().Get("Content-Type")
        l.log(route, method, scheme, host, contentType, body, captureWriter.body.Bytes(), responseContentType)
    }
}
```

#### Log Function (server/inference_request_log.go)

```go
func (l *inferenceRequestLogger) log(..., responseBody []byte, responseContentType string) {
    // Determine extension
    responseExt := "json"
    if strings.Contains(responseContentType, "ndjson") || strings.Contains(responseContentType, "event-stream") {
        responseExt = "jsonl"
    }

    // Convert SSE to JSONL if needed
    responseDataToLog := responseBody
    if strings.Contains(responseContentType, "event-stream") {
        responseDataToLog = extractJSONFromSSE(responseBody)
    }

    // Write files
    os.WriteFile(responsePath, responseDataToLog, 0o600)
}
```

## Testing

### Test Coverage

| Test | What it verifies |
|------|-----------------|
| `TestInferenceRequestLoggerMiddlewareWritesReplayArtifacts` | Existing test updated to assert response file exists |
| `TestResponseCaptureNonStreaming` | Single JSON response captured correctly with `.json` extension |
| `TestResponseCaptureStreaming` | NDJSON streaming response captured with `.jsonl` extension |
| `TestResponseCaptureError` | Error response (AbortWithStatusJSON) captured correctly |
| `TestExtractJSONFromSSE` | SSE `data: ` prefix stripping produces valid JSONL |

### Running Tests

```bash
go test ./server/ -run "TestInferenceRequestLogger|TestResponseCapture|TestExtractJSONFromSSE" -v
```

## Usage Example

### Enable Debug Logging

```bash
export OLLAMA_DEBUG_LOG_REQUESTS=true
ollama serve
```

### Make a Request

```bash
curl -X POST http://localhost:11434/api/generate \
  -H "Content-Type: application/json" \
  -d '{"model":"llama3","prompt":"Hello","stream":true}'
```

### Check Logged Files

```bash
# Find the log directory from the server output:
# "request debug logging enabled; inference request logs will be stored in /tmp/ollama-request-logs-123..."

ls /tmp/ollama-request-logs-123/

# View request body:
cat 20260525T150405.123456789Z-000001_api_generate_body.json

# View response stream:
cat 20260525T150405.123456789Z-000001_api_generate_response.jsonl

# Replay the request:
sh 20260525T150405.123456789Z-000001_api_generate_request.sh
```

## Design Decisions

### Why wrap c.Writer instead of intercepting handler return values?

The response writer wrapper captures the exact bytes sent to the client, including any transformations done by middleware (e.g., OpenAI's SSE conversion). Intercepting internal objects would miss these transformations.

### Why convert SSE to JSONL instead of logging raw SSE?

Raw SSE includes `data: ` prefixes and double newlines that are not valid JSON or JSONL. Converting to JSONL makes the logged files directly parseable by JSON tools and consistent with Ollama API streaming format.

### Why use the response Content-Type to determine extension?

The Content-Type header is set by the handler/middleware and accurately reflects the response format. Checking this after `c.Next()` ensures we know whether the response was streaming or not.

### Why not add a separate env var for response logging?

Response logging is only useful alongside request logging. Adding a separate toggle would complicate the feature without clear benefit. Both are gated by the same `OLLAMA_DEBUG_LOG_REQUESTS` flag.

## Porting to Another Branch

To apply this feature to a different branch or version of Ollama:

1. Ensure `server/inference_request_log.go` exists with the `inferenceRequestLogger` struct and `middleware()` function
2. Add the `responseCaptureWriter` struct and methods
3. Modify `middleware()` to wrap `c.Writer` before `c.Next()`
4. Extend `log()` to accept `responseBody` and `responseContentType` parameters
5. Add `extractJSONFromSSE()` function
6. Update tests in `server/routes_request_log_test.go`
7. Update envconfig description

## Known Limitations

- Logs are written to a temp directory that may be cleaned by the OS
- Large streaming responses are buffered entirely in memory before writing
- No rotation or size limits on individual log files
- Does not log non-inference routes (model management, etc.)
