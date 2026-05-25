package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ollama/ollama/envconfig"
)

type inferenceRequestLogger struct {
	dir     string
	counter uint64
}

type responseCaptureWriter struct {
	http.ResponseWriter
	body    bytes.Buffer
	status  int
	written bool
}

func (w *responseCaptureWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.written = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseCaptureWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	w.written = true
	return w.ResponseWriter.Write(data)
}

func (w *responseCaptureWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	w.written = true
	n, err := w.ResponseWriter.Write([]byte(s))
	return n, err
}

func (w *responseCaptureWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseCaptureWriter) Status() int {
	return w.status
}

func (w *responseCaptureWriter) Written() bool {
	return w.written
}

func (w *responseCaptureWriter) CloseNotify() <-chan bool {
	if notifier, ok := w.ResponseWriter.(http.CloseNotifier); ok {
		return notifier.CloseNotify()
	}
	return nil
}

func (w *responseCaptureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

func (w *responseCaptureWriter) Pusher() (pusher http.Pusher) {
	if p, ok := w.ResponseWriter.(interface{ Pusher() http.Pusher }); ok {
		return p.Pusher()
	}
	return nil
}

func (w *responseCaptureWriter) Size() int {
	return w.body.Len()
}

func (w *responseCaptureWriter) WriteHeaderNow() {
	if !w.written {
		w.WriteHeader(w.status)
	}
}

func newInferenceRequestLogger() (*inferenceRequestLogger, error) {
	dir, err := os.MkdirTemp("", "ollama-request-logs-*")
	if err != nil {
		return nil, err
	}

	return &inferenceRequestLogger{dir: dir}, nil
}

func (s *Server) initRequestLogging() error {
	if !envconfig.DebugLogRequests() {
		return nil
	}

	requestLogger, err := newInferenceRequestLogger()
	if err != nil {
		return fmt.Errorf("enable OLLAMA_DEBUG_LOG_REQUESTS: %w", err)
	}

	s.requestLogger = requestLogger
	slog.Info(fmt.Sprintf("request debug logging enabled; inference request logs will be stored in %s and include request bodies and replay curl commands", requestLogger.dir))

	return nil
}

func (s *Server) withInferenceRequestLogging(route string, handlers ...gin.HandlerFunc) []gin.HandlerFunc {
	if s.requestLogger == nil {
		return handlers
	}

	return append([]gin.HandlerFunc{s.requestLogger.middleware(route)}, handlers...)
}

func (l *inferenceRequestLogger) middleware(route string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil {
			c.Next()
			return
		}

		method := c.Request.Method
		host := c.Request.Host
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		contentType := c.GetHeader("Content-Type")

		var body []byte
		if c.Request.Body != nil {
			var err error
			body, err = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				slog.Warn("failed to read request body for debug logging", "route", route, "error", err)
			}
		}

		originalWriter := c.Writer
		captureWriter := &responseCaptureWriter{ResponseWriter: originalWriter, status: http.StatusOK}
		c.Writer = captureWriter

		c.Next()
		responseContentType := c.Writer.Header().Get("Content-Type")
		l.log(route, method, scheme, host, contentType, body, captureWriter.body.Bytes(), responseContentType)
	}
}

func (l *inferenceRequestLogger) log(route, method, scheme, host, contentType string, body []byte, responseBody []byte, responseContentType string) {
	if l == nil || l.dir == "" {
		return
	}

	if contentType == "" {
		contentType = "application/json"
	}
	if host == "" || scheme == "" {
		base := envconfig.Host()
		if host == "" {
			host = base.Host
		}
		if scheme == "" {
			scheme = base.Scheme
		}
	}

	routeForFilename := sanitizeRouteForFilename(route)
	timestamp := fmt.Sprintf("%s-%06d", time.Now().UTC().Format("20060102T150405.000000000Z"), atomic.AddUint64(&l.counter, 1))
	bodyFilename := fmt.Sprintf("%s_%s_body.json", timestamp, routeForFilename)
	curlFilename := fmt.Sprintf("%s_%s_request.sh", timestamp, routeForFilename)

	// Use .jsonl for streaming responses (NDJSON or SSE), .json for single-object responses
	responseExt := "json"
	if strings.Contains(responseContentType, "ndjson") || strings.Contains(responseContentType, "event-stream") {
		responseExt = "jsonl"
	}
	// For SSE responses, strip "data: " prefixes to produce valid JSONL
	responseDataToLog := responseBody
	if strings.Contains(responseContentType, "event-stream") {
		responseDataToLog = extractJSONFromSSE(responseBody)
	}
	responseFilename := fmt.Sprintf("%s_%s_response.%s", timestamp, routeForFilename, responseExt)

	bodyPath := filepath.Join(l.dir, bodyFilename)
	curlPath := filepath.Join(l.dir, curlFilename)
	responsePath := filepath.Join(l.dir, responseFilename)

	if err := os.WriteFile(bodyPath, body, 0o600); err != nil {
		slog.Warn("failed to write debug request body", "route", route, "error", err)
		return
	}

	if err := os.WriteFile(responsePath, responseDataToLog, 0o600); err != nil {
		slog.Warn("failed to write debug response body", "route", route, "error", err)
		return
	}

	url := fmt.Sprintf("%s://%s%s", scheme, host, route)
	curl := fmt.Sprintf("#!/bin/sh\nSCRIPT_DIR=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\"\ncurl --request %s --url %q --header %q --data-binary @\"${SCRIPT_DIR}/%s\"\n", method, url, "Content-Type: "+contentType, bodyFilename)
	if err := os.WriteFile(curlPath, []byte(curl), 0o600); err != nil {
		slog.Warn("failed to write debug request replay command", "route", route, "error", err)
		return
	}

	slog.Info(fmt.Sprintf("logged to %s and %s, replay using curl with `sh %s`", bodyPath, responsePath, curlPath))
}

func sanitizeRouteForFilename(route string) string {
	route = strings.TrimPrefix(route, "/")
	if route == "" {
		return "root"
	}

	var b strings.Builder
	b.Grow(len(route))
	for _, r := range route {
		if ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}

	return b.String()
}

// extractJSONFromSSE strips SSE "data: " prefixes from streaming responses
// to produce valid JSONL. It ignores "data: [DONE]" and comment/other event lines.
func extractJSONFromSSE(data []byte) []byte {
	var result bytes.Buffer
	for _, line := range bytes.Split(data, []byte("\n")) {
		lineStr := strings.TrimSpace(string(line))
		if strings.HasPrefix(lineStr, "data: ") {
			jsonData := strings.TrimPrefix(lineStr, "data: ")
			if jsonData != "[DONE]" {
				result.WriteString(jsonData)
				result.WriteByte('\n')
			}
		}
	}
	return result.Bytes()
}
