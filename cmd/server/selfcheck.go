package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
)

func runSelfcheck(ctx context.Context, address string) error {
	client := selfcheckClient{baseURL: "http://" + address, client: &http.Client{Timeout: 3 * time.Second}}
	caseID := "selfcheck-case"
	archivist := map[string]string{"id": "archivist-1", "role": "archivist"}
	reviewer := map[string]string{"id": "reviewer-1", "role": "reviewer"}
	manager := map[string]string{"id": "manager-1", "role": "manager"}
	now := time.Now().UTC()
	command := func(name, key string, revision uint64, actor map[string]string, payload any, expected int) (application.CommandResult, error) {
		body := map[string]any{"command": name, "expectedRevision": revision, "idempotencyKey": key, "actor": actor, "payload": payload}
		var result application.CommandResult
		err := client.command(ctx, caseID, body, expected, &result)
		return result, err
	}
	steps := []struct {
		name, key string
		revision  uint64
		actor     map[string]string
		payload   any
		expected  int
	}{
		{"create_case", "sc-01", 0, archivist, map[string]any{"title": "自检口述案卷", "languageCode": "zh", "collectionContext": "端到端自检", "ownerID": "archivist-1"}, http.StatusCreated},
		{"update_case_metadata", "sc-02", 1, archivist, map[string]any{"title": "自检口述案卷（已核对）"}, http.StatusOK},
		{"add_recording", "sc-03", 2, archivist, map[string]any{"recordingID": "rec-1", "label": "访谈录音", "durationMillis": 60000, "evidenceRef": "local:selfcheck"}, http.StatusOK},
		{"add_consent", "sc-04", 3, archivist, map[string]any{"consentID": "consent-1", "speakerID": "speaker-1", "scope": []string{"research_release"}, "restrictions": []string{}, "validFrom": now.Add(-time.Hour), "validUntil": now.Add(45 * 24 * time.Hour), "evidenceRef": "consent:selfcheck"}, http.StatusOK},
		{"batch_add_segments", "sc-05", 4, archivist, map[string]any{"segments": []map[string]any{{"segmentID": "seg-1", "recordingID": "rec-1", "speakerID": "speaker-1", "startMillis": 1000, "endMillis": 5000, "sourceText": "我叫阿明，住在青山村。", "sequence": 1}}}, http.StatusOK},
		{"lock_intake", "sc-06", 5, archivist, map[string]any{}, http.StatusOK},
		{"set_marks", "sc-07", 6, archivist, map[string]any{"segmentID": "seg-1", "marks": []map[string]any{{"markID": "mark-person", "category": "person", "startRune": 2, "endRune": 4, "action": "mask", "replacementText": "", "rationale": "直接身份标识", "resolutionStatus": "resolved"}}, "findingRefs": []any{}}, http.StatusOK},
		{"generate_redaction", "sc-08", 7, archivist, map[string]any{}, http.StatusOK},
		{"submit_review", "sc-09", 8, archivist, map[string]any{}, http.StatusOK},
		{"decide_review", "sc-10", 9, reviewer, map[string]any{"decisionID": "decision-return", "reviewerID": "reviewer-1", "sampledSegmentIDs": []string{"seg-1"}, "findings": []map[string]any{{"segmentID": "seg-1", "markID": "", "code": "place_exposed", "comment": "村名需要泛化"}}, "outcome": "returned"}, http.StatusOK},
		{"set_marks", "sc-11", 10, archivist, map[string]any{"segmentID": "seg-1", "marks": []map[string]any{{"markID": "mark-person", "category": "person", "startRune": 2, "endRune": 4, "action": "mask", "replacementText": "", "rationale": "直接身份标识", "resolutionStatus": "resolved"}, {"markID": "mark-place", "category": "place", "startRune": 7, "endRune": 10, "action": "generalize", "replacementText": "某村", "rationale": "复核要求降低地点精度", "resolutionStatus": "resolved"}}, "findingRefs": []map[string]any{{"reviewRound": 1, "segmentID": "seg-1", "markID": "", "code": "place_exposed"}}}, http.StatusOK},
		{"resolve_review_finding", "sc-12", 11, archivist, map[string]any{"reviewRound": 1, "segmentID": "seg-1", "markID": "", "code": "place_exposed", "correctionNote": "已将村名泛化为某村"}, http.StatusOK},
		{"generate_redaction", "sc-13", 12, archivist, map[string]any{}, http.StatusOK},
		{"submit_review", "sc-14", 13, archivist, map[string]any{}, http.StatusOK},
		{"decide_review", "sc-15", 14, reviewer, map[string]any{"decisionID": "decision-approve", "reviewerID": "reviewer-1", "sampledSegmentIDs": []string{"seg-1"}, "findings": []map[string]any{}, "outcome": "approved"}, http.StatusOK},
		{"release", "sc-16", 15, manager, map[string]any{"issuedBy": "manager-1"}, http.StatusOK},
	}
	var released application.CommandResult
	for _, step := range steps {
		result, err := command(step.name, step.key, step.revision, step.actor, step.payload, step.expected)
		if err != nil {
			return fmt.Errorf("自检命令 %s 失败: %w", step.name, err)
		}
		released = result
	}
	if released.Credential == nil {
		return fmt.Errorf("放行响应缺少开放凭据")
	}
	var verification application.CredentialVerification
	if err := client.get(ctx, "/api/v1/credentials/"+released.Credential.CredentialID+"/verify", http.StatusOK, &verification); err != nil {
		return err
	}
	if !verification.Valid {
		return fmt.Errorf("开放凭据校验未通过")
	}
	frozenBody := map[string]any{"command": "set_marks", "expectedRevision": 16, "idempotencyKey": "sc-frozen", "actor": archivist, "payload": map[string]any{"segmentID": "seg-1", "marks": []any{}, "findingRefs": []any{}}}
	if err := client.command(ctx, caseID, frozenBody, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("冻结不可变检查失败: %w", err)
	}
	var view application.CaseView
	if err := client.get(ctx, "/api/v1/cases/"+caseID, http.StatusOK, &view); err != nil {
		return err
	}
	if view.Case.Status != "released" || view.Case.Revision != 16 {
		return fmt.Errorf("自检案卷最终状态或修订号不正确")
	}
	return nil
}
