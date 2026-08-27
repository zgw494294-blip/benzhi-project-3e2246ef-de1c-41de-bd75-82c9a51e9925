package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request = withRequestID(request)
		writer.Header().Set("X-Request-ID", requestIDFrom(request.Context()))
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: writer}
		next.ServeHTTP(wrapped, request)
		logger.Info("http_access", "request_id", requestIDFrom(request.Context()), "method", request.Method, "path", request.URL.Path, "status", wrapped.status, "duration_ms", time.Since(started).Milliseconds())
	})
}
