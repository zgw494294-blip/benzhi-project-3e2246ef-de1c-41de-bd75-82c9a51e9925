package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

type Repository struct {
	mu           sync.RWMutex
	directory    string
	log          eventLog
	snapshotPath string
	state        projection
}

type CommitRequest struct {
	Case             *domain.CorpusCase
	Event            domain.Event
	ExpectedRevision uint64
	IdempotencyKey   string
	RequestHash      string
	Response         json.RawMessage
	Manifest         *domain.FrozenManifest
	Credential       *domain.ReleaseCredential
}

func Open(directory string) (*Repository, error) {
	if directory == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	repository := &Repository{
		directory: directory, log: eventLog{path: filepath.Join(directory, "events.log")},
		snapshotPath: filepath.Join(directory, "projection.json"),
	}
	state, err := recoverProjection(repository.log, repository.snapshotPath)
	if err != nil {
		return nil, err
	}
	repository.state = state
	if err := reconcileImmutable(directory, &state); err != nil {
		return nil, err
	}
	return repository, nil
}

func (r *Repository) GetCase(caseID string) (*domain.CorpusCase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.state.Cases[caseID]
	if !ok {
		return nil, domain.NotFound("案卷", caseID)
	}
	return c.Clone(), nil
}

func (r *Repository) GetCached(caseID, key, requestHash string) (json.RawMessage, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cached, ok := r.state.Idempotency[idempotencyIndex(caseID, key)]
	if !ok {
		return nil, false, nil
	}
	if cached.RequestHash != requestHash {
		return nil, false, domain.Conflict("idempotency_mismatch", "相同 idempotencyKey 对应了不同请求")
	}
	return append(json.RawMessage(nil), cached.Response...), true, nil
}

func (r *Repository) Commit(request CommitRequest) error {
	if request.Case == nil || request.IdempotencyKey == "" {
		return fmt.Errorf("提交缺少案卷或幂等键")
	}
	if err := request.Case.ValidateProjection(); err != nil {
		return fmt.Errorf("案卷投影校验失败: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.state.Cases[request.Case.CaseID]
	currentRevision := uint64(0)
	if current != nil {
		currentRevision = current.Revision
	}
	if currentRevision != request.ExpectedRevision {
		return domain.Conflict("revision_conflict", "修订号冲突：当前为 %d，期望为 %d", currentRevision, request.ExpectedRevision)
	}
	if request.Case.Revision != currentRevision+1 || request.Event.Revision != request.Case.Revision {
		return fmt.Errorf("领域提交必须恰好推进一个修订号")
	}
	index := idempotencyIndex(request.Case.CaseID, request.IdempotencyKey)
	if existing, ok := r.state.Idempotency[index]; ok {
		if existing.RequestHash != request.RequestHash {
			return domain.Conflict("idempotency_mismatch", "相同幂等键对应了不同请求")
		}
		return nil
	}
	if request.Credential != nil {
		if _, exists := r.state.Credentials[request.Credential.CredentialID]; exists {
			return domain.Conflict("credential_exists", "开放凭据不可覆盖")
		}
		if _, exists := r.state.Manifests[request.Case.CaseID]; exists {
			return domain.Conflict("manifest_exists", "冻结清单不可覆盖")
		}
	}
	cached := CachedResult{CaseID: request.Case.CaseID, IdempotencyKey: request.IdempotencyKey, RequestHash: request.RequestHash, Response: append(json.RawMessage(nil), request.Response...), StoredAt: time.Now().UTC()}
	record := eventRecord{Sequence: r.state.LastSequence + 1, Event: request.Event, Case: request.Case.Clone(), Cached: cached, Manifest: request.Manifest, Credential: request.Credential}
	if err := r.log.append(record); err != nil {
		return err
	}
	applyRecord(&r.state, record)
	if request.Manifest != nil {
		if err := appendImmutable(filepath.Join(r.directory, "manifests.jsonl"), "manifest", request.Manifest.CaseID, request.Manifest); err != nil {
			return err
		}
	}
	if request.Credential != nil {
		if err := appendImmutable(filepath.Join(r.directory, "credentials.jsonl"), "credential", request.Credential.CredentialID, request.Credential); err != nil {
			return err
		}
	}
	return writeSnapshot(r.snapshotPath, r.state)
}

func (r *Repository) GetCredential(id string) (domain.ReleaseCredential, domain.FrozenManifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	credential, ok := r.state.Credentials[id]
	if !ok {
		return domain.ReleaseCredential{}, domain.FrozenManifest{}, domain.NotFound("开放凭据", id)
	}
	manifest, ok := r.state.Manifests[credential.CaseID]
	if !ok {
		return domain.ReleaseCredential{}, domain.FrozenManifest{}, fmt.Errorf("凭据缺少冻结清单")
	}
	return credential, manifest, nil
}

func (r *Repository) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return writeSnapshot(r.snapshotPath, r.state)
}
