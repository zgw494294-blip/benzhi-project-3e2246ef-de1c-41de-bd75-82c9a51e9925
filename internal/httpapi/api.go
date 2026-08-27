package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
)

type API struct {
	service *application.Service
	logger  *slog.Logger
}

func New(service *application.Service, logger *slog.Logger) http.Handler {
	api := &API{service: service, logger: logger}
	return accessLog(logger, http.HandlerFunc(api.route))
}

func (a *API) route(writer http.ResponseWriter, request *http.Request) {
	applySecurityHeaders(writer)
	path := strings.Trim(request.URL.Path, "/")
	parts := strings.Split(path, "/")
	if request.Method == http.MethodGet && path == "healthz" {
		a.Health(writer, request)
		return
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "cases" && parts[4] == "commands" && request.Method == http.MethodPost {
		a.Commands(writer, request, parts[3])
		return
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "cases" && request.Method == http.MethodGet {
		a.Case(writer, request, parts[3])
		return
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "credentials" && parts[4] == "verify" && request.Method == http.MethodGet {
		a.VerifyCredential(writer, request, parts[3])
		return
	}
	if routeExistsForOtherMethod(parts, path) {
		writeProtocolError(writer, request, http.StatusMethodNotAllowed, "method_not_allowed", "该路由不支持当前 HTTP 方法")
		return
	}
	writeProtocolError(writer, request, http.StatusNotFound, "route_not_found", "未找到请求路由")
}

func applySecurityHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Frame-Options", "DENY")
}

func routeExistsForOtherMethod(parts []string, path string) bool {
	if path == "healthz" {
		return true
	}
	if len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "cases" {
		return true
	}
	if len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "cases" && parts[4] == "commands" {
		return true
	}
	return len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "credentials" && parts[4] == "verify"
}
