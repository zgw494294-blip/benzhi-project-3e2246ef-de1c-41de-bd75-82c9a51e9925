package httpapi

import (
	"context"
	"net/http"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
)

type commandOutcome struct {
	result application.CommandResult
	err    error
}

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
	outcomes := make(chan commandOutcome, 1)
	commandContext := context.WithoutCancel(request.Context())
	go func() {
		result, err := a.service.Execute(commandContext, command)
		outcomes <- commandOutcome{result: result, err: err}
	}()
	if err := request.Context().Err(); err != nil {
		<-outcomes
		writeError(writer, request, err)
		return
	}
	var outcome commandOutcome
	select {
	case outcome = <-outcomes:
	case <-request.Context().Done():
		<-outcomes
		writeError(writer, request, request.Context().Err())
		return
	}
	if outcome.err != nil {
		writeError(writer, request, outcome.err)
		return
	}
	status := http.StatusOK
	if command.Command == "create_case" && !outcome.result.IdempotentReplay {
		status = http.StatusCreated
	}
	writeSuccess(writer, status, outcome.result)
}
