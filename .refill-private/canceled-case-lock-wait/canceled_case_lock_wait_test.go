package canceled_case_lock_wait_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

type boundaryContext struct {
	mu       sync.Mutex
	checks   int
	err      error
	done     chan struct{}
	atWait   chan struct{}
	waitOnce sync.Once
	doneOnce sync.Once
}

func newBoundaryContext() *boundaryContext {
	return &boundaryContext{done: make(chan struct{}), atWait: make(chan struct{})}
}

func (c *boundaryContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *boundaryContext) Done() <-chan struct{}       { return c.done }
func (c *boundaryContext) Value(any) any               { return nil }

func (c *boundaryContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks++
	if c.checks == 2 {
		c.waitOnce.Do(func() { close(c.atWait) })
	}
	return c.err
}

func (c *boundaryContext) cancel() {
	c.mu.Lock()
	c.err = context.Canceled
	c.mu.Unlock()
	c.doneOnce.Do(func() { close(c.done) })
}

type executeResult struct {
	result application.CommandResult
	err    error
}

func TestCanceledCommandWaitingForCaseLockDoesNotCommit(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, redaction.New())

	eventPath := filepath.Join(directory, "events.log")
	if err := syscall.Mkfifo(eventPath, 0600); err != nil {
		t.Fatal(err)
	}
	first := application.CommandRequest{
		CaseID: "cancel-case", Command: "create_case", ExpectedRevision: 0,
		IdempotencyKey: "first", Actor: application.Actor{ID: "archivist", Role: "archivist"},
		Payload: json.RawMessage(`{"title":"` + strings.Repeat("占", 1<<17) + `","languageCode":"zh","collectionContext":"受控锁等待","ownerID":"archivist"}`),
	}
	firstDone := make(chan error, 1)
	go func() {
		_, executeErr := service.Execute(context.Background(), first)
		firstDone <- executeErr
	}()

	reader, err := os.OpenFile(eventPath, os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	blockedPath := filepath.Join(directory, "blocked-events.pipe")
	if err := os.Rename(eventPath, blockedPath); err != nil {
		t.Fatal(err)
	}
	regular, err := os.OpenFile(eventPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := regular.Close(); err != nil {
		t.Fatal(err)
	}

	waitContext := newBoundaryContext()
	second := first
	second.IdempotencyKey = "canceled"
	second.Payload = json.RawMessage(`{"title":"不应提交的案卷","languageCode":"zh","collectionContext":"请求已取消","ownerID":"archivist"}`)
	secondDone := make(chan executeResult, 1)
	go func() {
		result, executeErr := service.Execute(waitContext, second)
		secondDone <- executeResult{result: result, err: executeErr}
	}()
	<-waitContext.atWait
	waitContext.cancel()

	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if err := <-firstDone; err == nil {
		t.Fatal("受控的首个命令应在命名管道 fsync 时失败")
	}
	secondOutcome := <-secondDone
	if !errors.Is(secondOutcome.err, context.Canceled) {
		t.Fatalf("已取消的锁等待命令仍返回 %+v, err=%v", secondOutcome.result, secondOutcome.err)
	}
	if _, err := repository.GetCase("cancel-case"); err == nil {
		t.Fatal("已取消的锁等待命令仍写入了案卷")
	}
}
