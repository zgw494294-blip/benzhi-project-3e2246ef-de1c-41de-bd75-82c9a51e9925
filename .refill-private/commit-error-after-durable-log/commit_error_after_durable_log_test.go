package commit_error_after_durable_log_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

func TestDurableCommandDoesNotReturnSnapshotFailure(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "projection.json"), 0700); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, redaction.New())
	request := application.CommandRequest{
		CaseID: "durable-case", Command: "create_case", ExpectedRevision: 0,
		IdempotencyKey: "create", Actor: application.Actor{ID: "archivist", Role: "archivist"},
		Payload: json.RawMessage(`{"title":"案卷","languageCode":"zh","collectionContext":"测试","ownerID":"owner"}`),
	}
	_, executeErr := service.Execute(context.Background(), request)
	stored, loadErr := repository.GetCase("durable-case")
	if executeErr != nil && loadErr == nil && stored.Revision == 1 {
		t.Fatalf("事件日志已经提交且内存状态可见，命令却返回失败: %v", executeErr)
	}
	if executeErr != nil {
		t.Fatal(executeErr)
	}
}
