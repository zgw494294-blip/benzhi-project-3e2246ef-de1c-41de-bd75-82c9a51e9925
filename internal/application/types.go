package application

import (
	"encoding/json"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
)

type Actor struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type CommandRequest struct {
	CaseID           string          `json:"-"`
	Command          string          `json:"command"`
	ExpectedRevision uint64          `json:"expectedRevision"`
	IdempotencyKey   string          `json:"idempotencyKey"`
	Actor            Actor           `json:"actor"`
	Payload          json.RawMessage `json:"payload"`
}

type CommandResult struct {
	CaseID           string                    `json:"caseID"`
	Status           domain.CaseStatus         `json:"status"`
	Revision         uint64                    `json:"revision"`
	EventType        string                    `json:"eventType"`
	IdempotentReplay bool                      `json:"idempotentReplay"`
	Credential       *domain.ReleaseCredential `json:"credential,omitempty"`
}

type CaseView struct {
	Case            *domain.CorpusCase       `json:"case"`
	Blockers        []redaction.Issue        `json:"blockers"`
	ConsentCoverage domain.ConsentCoverage   `json:"consentCoverage"`
	ReleasePreview  redaction.ReleasePreview `json:"releasePreview"`
}

type CredentialVerification struct {
	Credential domain.ReleaseCredential `json:"credential"`
	Valid      bool                     `json:"valid"`
	Reason     string                   `json:"reason"`
}
