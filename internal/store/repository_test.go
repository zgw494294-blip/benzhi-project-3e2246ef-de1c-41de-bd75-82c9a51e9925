package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

func TestRepositoryReplaysAndKeepsIdempotency(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c, event, err := domain.NewCase("c", "案卷", "zh", "", "owner", "actor", now)
	if err != nil {
		t.Fatal(err)
	}
	response := json.RawMessage(`{"revision":1}`)
	if err := repository.Commit(CommitRequest{Case: c, Event: event, ExpectedRevision: 0, IdempotencyKey: "key", RequestHash: "hash", Response: response}); err != nil {
		t.Fatal(err)
	}
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.GetCase("c")
	if err != nil || loaded.Revision != 1 {
		t.Fatalf("恢复案卷失败: %v", err)
	}
	cached, found, err := reopened.GetCached("c", "key", "hash")
	var cachedValue map[string]any
	decodeErr := json.Unmarshal(cached, &cachedValue)
	if err != nil || decodeErr != nil || !found || cachedValue["revision"] != float64(1) {
		t.Fatalf("恢复幂等结果失败: %v", err)
	}
}

func TestIncompleteTailFrameIsIgnored(t *testing.T) {
	directory := t.TempDir()
	repository, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c, event, _ := domain.NewCase("c", "案卷", "zh", "", "owner", "actor", now)
	if err := repository.Commit(CommitRequest{Case: c, Event: event, ExpectedRevision: 0, IdempotencyKey: "key", RequestHash: "hash", Response: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(directory, "events.log"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte{0, 0, 0, 20, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.GetCase("c")
	if err != nil {
		t.Fatal(err)
	}
	secondEvent, err := loaded.AddRecording(domain.Recording{RecordingID: "r", Label: "恢复后录音", DurationMS: 1000}, "actor", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Commit(CommitRequest{Case: loaded, Event: secondEvent, ExpectedRevision: 1, IdempotencyKey: "key-2", RequestHash: "hash-2", Response: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	again, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	finalCase, err := again.GetCase("c")
	if err != nil || finalCase.Revision != 2 {
		t.Fatalf("截断尾帧后再次提交未能恢复: %v", err)
	}
}
