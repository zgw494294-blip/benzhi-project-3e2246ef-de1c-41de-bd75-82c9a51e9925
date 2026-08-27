package application

import (
	"context"
	"encoding/json"
	"testing"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

func TestExecuteIdempotentReplayAndMismatch(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, redaction.New())
	request := CommandRequest{CaseID: "c", Command: "create_case", ExpectedRevision: 0, IdempotencyKey: "same", Actor: Actor{ID: "a", Role: "archivist"}, Payload: json.RawMessage(`{"title":"案卷","languageCode":"zh","collectionContext":"测试","ownerID":"a"}`)}
	first, err := service.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || !second.IdempotentReplay {
		t.Fatal("幂等重放标志不正确")
	}
	request.Payload = json.RawMessage(`{"title":"其他","languageCode":"zh","collectionContext":"测试","ownerID":"a"}`)
	if _, err := service.Execute(context.Background(), request); err == nil {
		t.Fatal("幂等键复用不同请求应失败")
	}
}

func TestUpdateMetadataReplayAndRevisionConflict(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, redaction.New())
	actor := Actor{ID: "archivist", Role: "archivist"}
	create := CommandRequest{
		CaseID: "metadata", Command: "create_case", ExpectedRevision: 0, IdempotencyKey: "create", Actor: actor,
		Payload: json.RawMessage(`{"title":"原标题","languageCode":"zh","collectionContext":"敏感采集背景","ownerID":"owner-a"}`),
	}
	if _, err := service.Execute(context.Background(), create); err != nil {
		t.Fatal(err)
	}
	update := CommandRequest{
		CaseID: "metadata", Command: "update_case_metadata", ExpectedRevision: 1, IdempotencyKey: "update", Actor: actor,
		Payload: json.RawMessage(`{"title":"新标题","ownerID":"owner-b"}`),
	}
	first, err := service.Execute(context.Background(), update)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := service.Execute(context.Background(), update)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 2 || !replay.IdempotentReplay || replay.Revision != first.Revision {
		t.Fatalf("元数据修订幂等结果不正确: %+v %+v", first, replay)
	}
	stale := update
	stale.IdempotencyKey = "stale"
	stale.Payload = json.RawMessage(`{"languageCode":"yue"}`)
	if _, err := service.Execute(context.Background(), stale); err == nil {
		t.Fatal("过期修订号应冲突")
	}
	view, err := service.GetCase(context.Background(), "metadata")
	if err != nil {
		t.Fatal(err)
	}
	if view.Case.Title != "新标题" || view.Case.OwnerID != "owner-b" || view.Case.LanguageCode != "zh" || view.Case.Revision != 2 || len(view.Case.Audit) != 2 {
		t.Fatalf("冲突或重放改变了案卷: %+v", view.Case)
	}
	facts := view.Case.Audit[1].Facts
	if _, exists := facts["collectionContext"]; exists {
		t.Fatal("元数据审计复制了采集背景原文")
	}
}
