package httpapi

import (
	"net/http"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
)

func (a *API) Commands(writer http.ResponseWriter, request *http.Request, caseID string) {
	if caseID == "" {
		writeProtocolError(writer, request, http.StatusBadRequest, "case_id_required", "caseID 不能为空")
		return
	}
	var command application.CommandRequest
	if err := decodeStrict(writer, request, &command); err != nil {
		writeProtocolError(writer, request, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	command.CaseID = caseID
	result, err := a.service.Execute(request.Context(), command)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	status := http.StatusOK
	if command.Command == "create_case" && !result.IdempotentReplay {
		status = http.StatusCreated
	}
	writeSuccess(writer, status, result)
}
