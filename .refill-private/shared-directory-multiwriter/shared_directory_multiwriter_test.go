package shared_directory_multiwriter_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

func TestSharedDirectoryWritersPreserveEventSequence(t *testing.T) {
	directory := t.TempDir()
	left, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	right, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	leftCase, leftEvent, err := domain.NewCase("left", "左案卷", "zh", "测试", "owner", "actor", now)
	if err != nil {
		t.Fatal(err)
	}
	rightCase, rightEvent, err := domain.NewCase("right", "右案卷", "zh", "测试", "owner", "actor", now)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errors := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	commit := func(repository *store.Repository, corpus *domain.CorpusCase, event domain.Event, key string) {
		ready.Done()
		<-start
		errors <- repository.Commit(store.CommitRequest{
			Case: corpus, Event: event, ExpectedRevision: 0, IdempotencyKey: key,
			RequestHash: key + "-hash", Response: json.RawMessage(`{"revision":1}`),
		})
	}
	go commit(left, leftCase, leftEvent, "left")
	go commit(right, rightCase, rightEvent, "right")
	ready.Wait()
	close(start)
	for index := 0; index < 2; index++ {
		if err := <-errors; err != nil {
			t.Fatalf("并发提交意外返回错误: %v", err)
		}
	}
	if _, err := store.Open(directory); err != nil {
		t.Fatalf("两个已成功返回的写入产生了重复事件序号，仓储无法重启: %v", err)
	}
}
