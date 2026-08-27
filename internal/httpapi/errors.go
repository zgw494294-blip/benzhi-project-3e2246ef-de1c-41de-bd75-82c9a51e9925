package httpapi

import (
	"context"
	"errors"
	"net/http"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

func (a *API) writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusInternalServerError
	response := apiError{Code: "internal_error", Message: "服务处理请求时发生内部错误", RequestID: requestIDFrom(request.Context())}
	var business *domain.BusinessError
	if errors.As(err, &business) {
		response.Code, response.Message, response.Details = business.Code, business.Message, business.Details
		switch business.Kind {
		case domain.KindValidation:
			status = http.StatusUnprocessableEntity
		case domain.KindConflict:
			status = http.StatusConflict
		case domain.KindNotFound:
			status = http.StatusNotFound
		case domain.KindForbidden:
			status = http.StatusForbidden
		}
	} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		status, response.Code, response.Message = http.StatusRequestTimeout, "request_timeout", "请求已取消或超时"
	}
	a.writeJSON(writer, status, errorEnvelope{Error: response})
}

func (a *API) writeProtocolError(writer http.ResponseWriter, request *http.Request, status int, code, message string) {
	a.writeJSON(writer, status, errorEnvelope{Error: apiError{Code: code, Message: message, RequestID: requestIDFrom(request.Context())}})
}
