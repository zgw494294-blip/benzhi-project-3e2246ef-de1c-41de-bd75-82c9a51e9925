package immutable_sidecar_overrides_log_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

type immutableRecord struct {
	Kind     string          `json:"kind"`
	ID       string          `json:"id"`
	Payload  json.RawMessage `json:"payload"`
	Checksum string          `json:"checksum"`
}

type manifestDigestDocument struct {
	CaseID string              `json:"caseID"`
	Items  []domain.FrozenItem `json:"items"`
}

func TestTamperedImmutableRecordsCannotOverrideEventLog(t *testing.T) {
	directory := t.TempDir()
	repository, err := store.Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repository, redaction.New())
	credentialID := releaseCase(t, service)
	if err := repository.Close(); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(directory, "manifests.jsonl")
	credentialPath := filepath.Join(directory, "credentials.jsonl")
	var manifest domain.FrozenManifest
	manifestRecord := readImmutable(t, manifestPath, &manifest)
	manifest.Items[0].Text = "伪造后仍被判定有效的公开文本"
	digestInput, err := json.Marshal(manifestDigestDocument{CaseID: manifest.CaseID, Items: manifest.Items})
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(digestInput)
	manifest.Digest = hex.EncodeToString(manifestDigest[:])
	writeImmutable(t, manifestPath, manifestRecord, manifest)

	var credential domain.ReleaseCredential
	credentialRecord := readImmutable(t, credentialPath, &credential)
	credential.ManifestDigest = manifest.Digest
	writeImmutable(t, credentialPath, credentialRecord, credential)

	reopened, err := store.Open(directory)
	if err != nil {
		return
	}
	verification, err := application.NewService(reopened, redaction.New()).VerifyCredential(context.Background(), credentialID)
	if err == nil && verification.Valid {
		t.Fatal("重启接受了与事件日志分叉的只追加记录，并把伪造开放凭据判为有效")
	}
}

func releaseCase(t *testing.T, service *application.Service) string {
	t.Helper()
	now := time.Now().UTC()
	archivist := application.Actor{ID: "archivist", Role: "archivist"}
	reviewer := application.Actor{ID: "reviewer", Role: "reviewer"}
	manager := application.Actor{ID: "manager", Role: "manager"}
	commands := []struct {
		name    string
		actor   application.Actor
		payload string
	}{
		{"create_case", archivist, `{"title":"私有复现案卷","languageCode":"zh","collectionContext":"完整性复现","ownerID":"archivist"}`},
		{"add_recording", archivist, `{"recordingID":"rec-1","label":"复现录音","durationMillis":60000,"evidenceRef":"local:test"}`},
		{"add_consent", archivist, mustJSON(t, map[string]any{"consentID": "consent-1", "speakerID": "speaker-1", "scope": []string{"research_release"}, "restrictions": []string{}, "validFrom": now.Add(-time.Hour), "validUntil": now.Add(24 * time.Hour), "evidenceRef": "consent:test"})},
		{"add_segment", archivist, `{"segmentID":"seg-1","recordingID":"rec-1","speakerID":"speaker-1","startMillis":1000,"endMillis":5000,"sourceText":"阿明住在村里","sequence":1}`},
		{"lock_intake", archivist, `{}`},
		{"set_marks", archivist, `{"segmentID":"seg-1","marks":[{"markID":"mark-1","category":"person","startRune":0,"endRune":2,"action":"mask","replacementText":"","rationale":"直接身份标识","resolutionStatus":"resolved"}],"findingRefs":[]}`},
		{"generate_redaction", archivist, `{}`},
		{"submit_review", archivist, `{}`},
		{"decide_review", reviewer, `{"decisionID":"decision-1","reviewerID":"reviewer","sampledSegmentIDs":["seg-1"],"findings":[],"outcome":"approved"}`},
		{"release", manager, `{"issuedBy":"manager"}`},
	}
	var result application.CommandResult
	for index, item := range commands {
		request := application.CommandRequest{
			CaseID: "case-1", Command: item.name, ExpectedRevision: uint64(index),
			IdempotencyKey: "key-" + item.name, Actor: item.actor, Payload: json.RawMessage(item.payload),
		}
		var err error
		result, err = service.Execute(context.Background(), request)
		if err != nil {
			t.Fatalf("准备放行状态的命令 %s 失败: %v", item.name, err)
		}
	}
	if result.Credential == nil {
		t.Fatal("准备放行状态未返回凭据")
	}
	return result.Credential.CredentialID
}

func readImmutable(t *testing.T, path string, target any) immutableRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record immutableRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(record.Payload, target); err != nil {
		t.Fatal(err)
	}
	return record
}

func writeImmutable(t *testing.T, path string, record immutableRecord, payload any) {
	t.Helper()
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	checksum := sha256.Sum256(encodedPayload)
	record.Payload = encodedPayload
	record.Checksum = hex.EncodeToString(checksum[:])
	encodedRecord, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	encodedRecord = append(encodedRecord, '\n')
	if err := os.WriteFile(path, encodedRecord, 0600); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
