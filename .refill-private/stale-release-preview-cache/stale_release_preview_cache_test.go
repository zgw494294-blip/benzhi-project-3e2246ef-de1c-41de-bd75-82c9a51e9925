package stale_release_preview_cache_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

func TestReleasePreviewCacheIsRevisionScopedAndIsolated(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, redaction.New())
	archivist := application.Actor{ID: "archivist-1", Role: "archivist"}
	reviewer := application.Actor{ID: "reviewer-1", Role: "reviewer"}
	manager := application.Actor{ID: "manager-1", Role: "manager"}
	caseID := "preview-cache-case"
	now := time.Now().UTC()

	mustExecute(t, service, caseID, "create_case", 0, "create", archivist, map[string]any{
		"title": "缓存隔离复现案卷", "languageCode": "zh", "collectionContext": "私有复现", "ownerID": archivist.ID,
	})
	mustExecute(t, service, caseID, "add_recording", 1, "recording", archivist, map[string]any{
		"recordingID": "rec-1", "label": "访谈", "durationMillis": 20_000, "evidenceRef": "local:recording",
	})
	mustExecute(t, service, caseID, "add_consent", 2, "consent", archivist, map[string]any{
		"consentID": "consent-1", "speakerID": "speaker-1", "scope": []string{"research_release"},
		"restrictions": []string{}, "validFrom": now.Add(-time.Hour), "validUntil": now.Add(24 * time.Hour),
		"evidenceRef": "local:consent",
	})
	mustExecute(t, service, caseID, "add_segment", 3, "segment", archivist, map[string]any{
		"segmentID": "seg-1", "recordingID": "rec-1", "speakerID": "speaker-1", "startMillis": 100,
		"endMillis": 1_000, "sourceText": "这是一段可开放的口述资料。", "sequence": 1,
	})
	mustExecute(t, service, caseID, "lock_intake", 4, "lock", archivist, map[string]any{})
	mustExecute(t, service, caseID, "generate_redaction", 5, "redact", archivist, map[string]any{})
	mustExecute(t, service, caseID, "submit_review", 6, "submit", archivist, map[string]any{})
	mustExecute(t, service, caseID, "decide_review", 7, "approve", reviewer, map[string]any{
		"decisionID": "decision-1", "reviewerID": reviewer.ID, "sampledSegmentIDs": []string{"seg-1"},
		"findings": []any{}, "outcome": "approved",
	})

	first, err := service.GetCase(context.Background(), caseID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Case.Status != domain.StatusReviewApproved || first.ReleasePreview.Status != "ready" || len(first.ReleasePreview.Items) != 1 {
		t.Fatalf("预检前置状态不正确: status=%s preview=%+v", first.Case.Status, first.ReleasePreview)
	}
	originalText := first.ReleasePreview.Items[0].Text
	first.ReleasePreview.Items[0].Text = "调用方污染的缓存文本"

	sameRevision, err := service.GetCase(context.Background(), caseID)
	if err != nil {
		t.Fatal(err)
	}
	sameRevisionPolluted := sameRevision.ReleasePreview.Items[0].Text != originalText

	mustExecute(t, service, caseID, "release", 8, "release", manager, map[string]any{"issuedBy": manager.ID})
	afterRelease, err := service.GetCase(context.Background(), caseID)
	if err != nil {
		t.Fatal(err)
	}
	if afterRelease.Case.Status != domain.StatusReleased {
		t.Fatalf("放行命令未推进状态: %s", afterRelease.Case.Status)
	}
	staleAcrossRevision := afterRelease.ReleasePreview.Available || afterRelease.ReleasePreview.Status != "unavailable" || len(afterRelease.ReleasePreview.Items) != 0
	if sameRevisionPolluted || staleAcrossRevision {
		t.Fatalf("预检缓存缺少所有权或修订隔离: sameRevisionPolluted=%t staleAcrossRevision=%t preview=%+v", sameRevisionPolluted, staleAcrossRevision, afterRelease.ReleasePreview)
	}
}

func mustExecute(t *testing.T, service *application.Service, caseID, command string, revision uint64, key string, actor application.Actor, payload any) application.CommandResult {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Execute(context.Background(), application.CommandRequest{
		CaseID: caseID, Command: command, ExpectedRevision: revision, IdempotencyKey: key, Actor: actor, Payload: encoded,
	})
	if err != nil {
		t.Fatalf("命令 %s 失败: %v", command, err)
	}
	return result
}
