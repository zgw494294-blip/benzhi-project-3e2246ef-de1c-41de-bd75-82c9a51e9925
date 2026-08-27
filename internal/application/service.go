package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

type Service struct {
	repository *store.Repository
	redactor   *redaction.Engine
	locks      *caseLocks
	previewMu  sync.RWMutex
	previews   map[string]redaction.ReleasePreview
	now        func() time.Time
}

func NewService(repository *store.Repository, redactor *redaction.Engine) *Service {
	return &Service{
		repository: repository,
		redactor:   redactor,
		locks:      newCaseLocks(),
		previews:   make(map[string]redaction.ReleasePreview),
		now:        time.Now,
	}
}

func (s *Service) Execute(ctx context.Context, request CommandRequest) (CommandResult, error) {
	if err := ctx.Err(); err != nil {
		return CommandResult{}, err
	}
	if request.CaseID == "" || request.IdempotencyKey == "" {
		return CommandResult{}, domain.Validation("command_fields_required", "caseID 和 idempotencyKey 不能为空")
	}
	if len(request.IdempotencyKey) > 128 {
		return CommandResult{}, domain.Validation("idempotency_key_too_long", "idempotencyKey 最长 128 字符")
	}
	unlock := s.locks.lock(request.CaseID)
	defer unlock()
	if err := authorize(request.Command, request.Actor); err != nil {
		return CommandResult{}, err
	}
	hash, err := hashRequest(request)
	if err != nil {
		return CommandResult{}, err
	}
	if cached, found, err := s.repository.GetCached(request.CaseID, request.IdempotencyKey, hash); err != nil {
		return CommandResult{}, err
	} else if found {
		var result CommandResult
		if err := json.Unmarshal(cached, &result); err != nil {
			return CommandResult{}, fmt.Errorf("解析幂等结果: %w", err)
		}
		result.IdempotentReplay = true
		return result, nil
	}
	return s.executeNew(request, hash)
}

func (s *Service) executeNew(request CommandRequest, requestHash string) (CommandResult, error) {
	now := s.now().UTC()
	var aggregate *domain.CorpusCase
	var event domain.Event
	var manifest *domain.FrozenManifest
	var credential *domain.ReleaseCredential
	var err error
	if request.Command == "create_case" {
		if request.ExpectedRevision != 0 {
			return CommandResult{}, domain.Conflict("revision_conflict", "创建案卷的 expectedRevision 必须为 0")
		}
		if _, loadErr := s.repository.GetCase(request.CaseID); loadErr == nil {
			return CommandResult{}, domain.Conflict("case_exists", "案卷已存在")
		}
		aggregate, event, err = s.createCase(request, now)
	} else {
		aggregate, err = s.repository.GetCase(request.CaseID)
		if err != nil {
			return CommandResult{}, err
		}
		if aggregate.Revision != request.ExpectedRevision {
			return CommandResult{}, domain.Conflict("revision_conflict", "修订号冲突：当前为 %d，期望为 %d", aggregate.Revision, request.ExpectedRevision)
		}
		aggregate = aggregate.Clone()
		event, manifest, credential, err = s.applyCommand(aggregate, request, now)
	}
	if err != nil {
		return CommandResult{}, err
	}
	result := CommandResult{CaseID: aggregate.CaseID, Status: aggregate.Status, Revision: aggregate.Revision, EventType: event.Type, Credential: credential}
	encoded, err := json.Marshal(result)
	if err != nil {
		return CommandResult{}, err
	}
	commit := store.CommitRequest{
		Case: aggregate, Event: event, ExpectedRevision: request.ExpectedRevision,
		IdempotencyKey: request.IdempotencyKey, RequestHash: requestHash, Response: encoded,
		Manifest: manifest, Credential: credential,
	}
	if err := s.repository.Commit(commit); err != nil {
		return CommandResult{}, err
	}
	return result, nil
}

func hashRequest(request CommandRequest) (string, error) {
	encoded, err := json.Marshal(struct {
		CaseID   string          `json:"caseID"`
		Command  string          `json:"command"`
		Expected uint64          `json:"expectedRevision"`
		Actor    Actor           `json:"actor"`
		Payload  json.RawMessage `json:"payload"`
	}{request.CaseID, request.Command, request.ExpectedRevision, request.Actor, request.Payload})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func decodePayload(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.Validation("invalid_payload", "命令 payload 无效：%v", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return domain.Validation("invalid_payload", "命令 payload 只能包含一个 JSON 对象")
	}
	return nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("存在额外 JSON 值")
	}
	return err
}
