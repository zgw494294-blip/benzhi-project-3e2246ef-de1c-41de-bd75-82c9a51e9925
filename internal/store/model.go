package store

import (
	"encoding/json"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

type CachedResult struct {
	CaseID         string          `json:"caseID"`
	IdempotencyKey string          `json:"idempotencyKey"`
	RequestHash    string          `json:"requestHash"`
	Response       json.RawMessage `json:"response"`
	StoredAt       time.Time       `json:"storedAt"`
}

type eventRecord struct {
	Sequence   uint64                    `json:"sequence"`
	Event      domain.Event              `json:"event"`
	Case       *domain.CorpusCase        `json:"case"`
	Cached     CachedResult              `json:"cached"`
	Manifest   *domain.FrozenManifest    `json:"manifest,omitempty"`
	Credential *domain.ReleaseCredential `json:"credential,omitempty"`
}

type projection struct {
	LastSequence uint64                              `json:"lastSequence"`
	Cases        map[string]*domain.CorpusCase       `json:"cases"`
	Idempotency  map[string]CachedResult             `json:"idempotency"`
	Manifests    map[string]domain.FrozenManifest    `json:"manifests"`
	Credentials  map[string]domain.ReleaseCredential `json:"credentials"`
}

func emptyProjection() projection {
	return projection{
		Cases: map[string]*domain.CorpusCase{}, Idempotency: map[string]CachedResult{},
		Manifests: map[string]domain.FrozenManifest{}, Credentials: map[string]domain.ReleaseCredential{},
	}
}

func idempotencyIndex(caseID, key string) string { return caseID + "\x00" + key }
