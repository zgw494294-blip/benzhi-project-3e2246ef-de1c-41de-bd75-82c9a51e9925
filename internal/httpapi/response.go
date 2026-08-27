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

func (a *API) writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	a.responseBuffer.Reset()
	if err := json.NewEncoder(&a.responseBuffer).Encode(value); err != nil {
		return
	}
	writer.WriteHeader(status)
	_, _ = writer.Write(a.responseBuffer.Bytes())
}

func (a *API) writeSuccess(writer http.ResponseWriter, status int, data any) {
	a.writeJSON(writer, status, successEnvelope{Data: data})
}
