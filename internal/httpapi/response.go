package httpapi

import (
	"encoding/json"
	"net/http"
)

type successEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestID"`
	Details   map[string]any `json:"details,omitempty"`
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeSuccess(writer http.ResponseWriter, status int, data any) {
	writeJSON(writer, status, successEnvelope{Data: data})
}
