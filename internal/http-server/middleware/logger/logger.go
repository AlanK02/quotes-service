package logger

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int
	headerWritten bool
}

func newResponseWriterInterceptor(w http.ResponseWriter) *responseWriterInterceptor {
	return &responseWriterInterceptor{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (wri *responseWriterInterceptor) WriteHeader(code int) {
	if wri.headerWritten {
		return
	}
	wri.ResponseWriter.WriteHeader(code)
	wri.statusCode = code
	wri.headerWritten = true 
}

func (wri *responseWriterInterceptor) Write(b []byte) (int, error) {
	if !wri.headerWritten {
		wri.WriteHeader(http.StatusOK)
	}
	n, err := wri.ResponseWriter.Write(b)
	wri.bytesWritten += n
	return n, err
}

func (wri *responseWriterInterceptor) Status() int {
	return wri.statusCode
}

func (wri *responseWriterInterceptor) BytesWritten() int {
	return wri.bytesWritten
}

func (wri *responseWriterInterceptor) Flush() {
	if flusher, ok := wri.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func generateRequestID(logForError *slog.Logger) string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		if logForError != nil {
			logForError.Error("Failed to generate secure request ID from crypto/rand", slog.String("error", err.Error()))
		}
		return "fallback_" + time.Now().Format(time.RFC3339Nano)
	}
	return hex.EncodeToString(bytes)
}

func New(log *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		middlewareLog := log.With(
			slog.String("component", "middleware/logger"),
		)

		middlewareLog.Info("logger middleware enabled")

		fn := func(w http.ResponseWriter, r *http.Request) {
			requestID := generateRequestID(middlewareLog)

			entry := middlewareLog.With(
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
				slog.String("request_id", requestID),
			)

			interceptor := newResponseWriterInterceptor(w)

			startTime := time.Now()
			defer func() {
				entry.Info("request completed",
					slog.Int("status", interceptor.Status()),
					slog.Int("bytes", interceptor.BytesWritten()),
					slog.Duration("duration", time.Since(startTime)),
				)
			}()

			next.ServeHTTP(interceptor, r)
		}
		return http.HandlerFunc(fn)
	}
}