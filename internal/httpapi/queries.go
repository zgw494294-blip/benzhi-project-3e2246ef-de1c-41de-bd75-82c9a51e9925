package httpapi

import "net/http"

func (a *API) Case(writer http.ResponseWriter, request *http.Request, caseID string) {
	view, err := a.service.GetCase(request.Context(), caseID)
	if err != nil {
		a.writeError(writer, request, err)
		return
	}
	a.writeSuccess(writer, http.StatusOK, view)
}

func (a *API) VerifyCredential(writer http.ResponseWriter, request *http.Request, credentialID string) {
	verification, err := a.service.VerifyCredential(request.Context(), credentialID)
	if err != nil {
		a.writeError(writer, request, err)
		return
	}
	a.writeSuccess(writer, http.StatusOK, verification)
}

func (a *API) Health(writer http.ResponseWriter, request *http.Request) {
	a.writeSuccess(writer, http.StatusOK, map[string]string{"status": "ok", "service": "口述语料脱敏放行台"})
}
