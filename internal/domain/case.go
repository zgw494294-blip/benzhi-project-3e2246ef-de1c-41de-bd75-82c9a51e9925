package domain

import (
	"strings"
	"time"
)

type CorpusCase struct {
	CaseID            string                       `json:"caseID"`
	Title             string                       `json:"title"`
	LanguageCode      string                       `json:"languageCode"`
	CollectionContext string                       `json:"collectionContext"`
	OwnerID           string                       `json:"ownerID"`
	Status            CaseStatus                   `json:"status"`
	Revision          uint64                       `json:"revision"`
	CreatedAt         time.Time                    `json:"createdAt"`
	UpdatedAt         time.Time                    `json:"updatedAt"`
	Recordings        map[string]Recording         `json:"recordings"`
	Consents          map[string]ConsentRecord     `json:"consents"`
	Segments          map[string]TranscriptSegment `json:"segments"`
	Marks             map[string][]RedactionMark   `json:"marks"`
	Redacted          map[string]RedactedSegment   `json:"redacted"`
	ReviewHistory     []ReviewDecision             `json:"reviewHistory"`
	ReviewFindings    []ReviewFinding              `json:"reviewFindings,omitempty"`
	OpenFindings      []ReviewFinding              `json:"openFindings,omitempty"`
	Manifest          *FrozenManifest              `json:"manifest,omitempty"`
	Credential        *ReleaseCredential           `json:"credential,omitempty"`
	Audit             []AuditFact                  `json:"audit"`
}

type MetadataPatch struct {
	Title             *string
	LanguageCode      *string
	CollectionContext *string
	OwnerID           *string
}

func (c *CorpusCase) UpdateMetadata(patch MetadataPatch, actor string, now time.Time) (Event, error) {
	if c.Status != StatusDraft {
		return Event{}, Conflict("metadata_update_status_conflict", "仅草稿案卷可修订元数据")
	}
	if patch.Title == nil && patch.LanguageCode == nil && patch.CollectionContext == nil && patch.OwnerID == nil {
		return Event{}, Validation("empty_metadata_patch", "元数据修订至少需要一个字段")
	}
	if patch.Title != nil && strings.TrimSpace(*patch.Title) == "" {
		return Event{}, Validation("metadata_title_required", "标题不能为空白")
	}
	if patch.LanguageCode != nil && strings.TrimSpace(*patch.LanguageCode) == "" {
		return Event{}, Validation("metadata_language_required", "语言不能为空白")
	}
	if patch.OwnerID != nil && strings.TrimSpace(*patch.OwnerID) == "" {
		return Event{}, Validation("metadata_owner_required", "负责人不能为空白")
	}
	changed := make([]string, 0, 4)
	if patch.Title != nil && c.Title != *patch.Title {
		c.Title = *patch.Title
		changed = append(changed, "title")
	}
	if patch.LanguageCode != nil && c.LanguageCode != *patch.LanguageCode {
		c.LanguageCode = *patch.LanguageCode
		changed = append(changed, "languageCode")
	}
	if patch.CollectionContext != nil && c.CollectionContext != *patch.CollectionContext {
		c.CollectionContext = *patch.CollectionContext
		changed = append(changed, "collectionContext")
	}
	if patch.OwnerID != nil && c.OwnerID != *patch.OwnerID {
		c.OwnerID = *patch.OwnerID
		changed = append(changed, "ownerID")
	}
	if len(changed) == 0 {
		return Event{}, Validation("metadata_unchanged", "元数据修订未产生任何变更")
	}
	return c.record("metadata.updated", actor, now, map[string]any{"changedFields": changed}), nil
}

func NewCase(id, title, language, context, owner, actor string, now time.Time) (*CorpusCase, Event, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(language) == "" || strings.TrimSpace(owner) == "" {
		return nil, Event{}, Validation("case_fields_required", "案卷 ID、标题、语言和负责人均不能为空")
	}
	c := &CorpusCase{
		CaseID: id, Title: title, LanguageCode: language, CollectionContext: context,
		OwnerID: owner, Status: StatusDraft, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		Recordings: map[string]Recording{}, Consents: map[string]ConsentRecord{},
		Segments: map[string]TranscriptSegment{}, Marks: map[string][]RedactionMark{},
		Redacted: map[string]RedactedSegment{}, ReviewHistory: []ReviewDecision{}, Audit: []AuditFact{},
	}
	e := c.record("case.created", actor, now, map[string]any{"title": title, "languageCode": language})
	return c, e, nil
}

func (c *CorpusCase) ensureMutable() error {
	if c.Status == StatusReleased || c.Manifest != nil {
		return Conflict("frozen_case", "开放材料已冻结，案卷不可修改")
	}
	return nil
}

func (c *CorpusCase) AddRecording(recording Recording, actor string, now time.Time) (Event, error) {
	if err := c.ensureMutable(); err != nil {
		return Event{}, err
	}
	if c.Status != StatusDraft {
		return Event{}, Conflict("intake_already_locked", "仅草稿案卷可登记录音")
	}
	if strings.TrimSpace(recording.RecordingID) == "" || recording.DurationMS <= 0 || strings.TrimSpace(recording.Label) == "" {
		return Event{}, Validation("invalid_recording", "录音 ID、标签和正数时长为必填项")
	}
	if _, exists := c.Recordings[recording.RecordingID]; exists {
		return Event{}, Conflict("recording_exists", "录音已存在")
	}
	c.Recordings[recording.RecordingID] = recording
	return c.record("recording.added", actor, now, map[string]any{"recordingID": recording.RecordingID}), nil
}
