package closedrepository_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

func TestClosedRepositoryRejectsEveryOperation(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	caseFile, created, err := domain.NewCase("closed-case", "关闭态案卷", "zh", "资源生命周期复现", "owner", "archivist", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.Commit(store.CommitRequest{
		Case: caseFile, Event: created, ExpectedRevision: 0,
		IdempotencyKey: "create", RequestHash: "create-hash", Response: json.RawMessage(`{"revision":1}`),
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	mutated := caseFile.Clone()
	newTitle := "关闭后不应持久化"
	updated, err := mutated.UpdateMetadata(domain.MetadataPatch{Title: &newTitle}, "archivist", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	commitErr := repository.Commit(store.CommitRequest{
		Case: mutated, Event: updated, ExpectedRevision: 1,
		IdempotencyKey: "after-close", RequestHash: "after-close-hash", Response: json.RawMessage(`{"revision":2}`),
	})
	_, _, credentialErr := repository.GetCredential("missing-credential")

	reopened, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	recovered, err := reopened.GetCase("closed-case")
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(commitErr, store.ErrRepositoryClosed) || !errors.Is(credentialErr, store.ErrRepositoryClosed) || recovered.Revision != 1 {
		t.Fatalf("关闭态所有权边界失效: commitErr=%v credentialErr=%v recoveredRevision=%d", commitErr, credentialErr, recovered.Revision)
	}
}
