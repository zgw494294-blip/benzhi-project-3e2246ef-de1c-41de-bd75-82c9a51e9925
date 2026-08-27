package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type contextKey string

const requestIDKey contextKey = "request-id"

func withRequestID(request *http.Request) *http.Request {
	id := strings.TrimSpace(request.Header.Get("X-Request-ID"))
	if id == "" || len(id) > 128 {
		id = newRequestID()
	}
	ctx := context.WithValue(request.Context(), requestIDKey, id)
	return request.WithContext(ctx)
}

func requestIDFrom(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func newRequestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "request-unavailable"
	}
	return "req_" + hex.EncodeToString(value)
}
