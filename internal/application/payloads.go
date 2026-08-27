package application

import (
	"encoding/json"
	"fmt"
	"time"
)

type OptionalString struct {
	Value string
	Set   bool
}

func (value *OptionalString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return fmt.Errorf("可选字符串字段不能为 null")
	}
	if err := json.Unmarshal(data, &value.Value); err != nil {
		return err
	}
	value.Set = true
	return nil
}

func (value OptionalString) Pointer() *string {
	if !value.Set {
		return nil
	}
	result := value.Value
	return &result
}

type CreateCasePayload struct {
	Title             string `json:"title"`
	LanguageCode      string `json:"languageCode"`
	CollectionContext string `json:"collectionContext"`
	OwnerID           string `json:"ownerID"`
}

type UpdateCaseMetadataPayload struct {
	Title             OptionalString `json:"title"`
	LanguageCode      OptionalString `json:"languageCode"`
	CollectionContext OptionalString `json:"collectionContext"`
	OwnerID           OptionalString `json:"ownerID"`
}

type AddRecordingPayload struct {
	RecordingID string `json:"recordingID"`
	Label       string `json:"label"`
	DurationMS  int64  `json:"durationMillis"`
	EvidenceRef string `json:"evidenceRef"`
}

type AddConsentPayload struct {
	ConsentID    string    `json:"consentID"`
	SpeakerID    string    `json:"speakerID"`
	Scope        []string  `json:"scope"`
	Restrictions []string  `json:"restrictions"`
	ValidFrom    time.Time `json:"validFrom"`
	ValidUntil   time.Time `json:"validUntil"`
	EvidenceRef  string    `json:"evidenceRef"`
}

type AddSegmentPayload struct {
	SegmentID   string `json:"segmentID"`
	RecordingID string `json:"recordingID"`
	SpeakerID   string `json:"speakerID"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	SourceText  string `json:"sourceText"`
	Sequence    int    `json:"sequence"`
}

type BatchAddSegmentsPayload struct {
	Segments []AddSegmentPayload `json:"segments"`
}

type SetMarksPayload struct {
	SegmentID   string              `json:"segmentID"`
	Marks       []MarkPayload       `json:"marks"`
	FindingRefs []FindingRefPayload `json:"findingRefs"`
}

type FindingRefPayload struct {
	ReviewRound int    `json:"reviewRound"`
	SegmentID   string `json:"segmentID"`
	MarkID      string `json:"markID"`
	Code        string `json:"code"`
}

type MarkPayload struct {
	MarkID           string `json:"markID"`
	Category         string `json:"category"`
	StartRune        int    `json:"startRune"`
	EndRune          int    `json:"endRune"`
	Action           string `json:"action"`
	ReplacementText  string `json:"replacementText"`
	Rationale        string `json:"rationale"`
	ResolutionStatus string `json:"resolutionStatus"`
}

type DecideReviewPayload struct {
	DecisionID        string           `json:"decisionID"`
	ReviewerID        string           `json:"reviewerID"`
	SampledSegmentIDs []string         `json:"sampledSegmentIDs"`
	Findings          []FindingPayload `json:"findings"`
	Outcome           string           `json:"outcome"`
}

type FindingPayload struct {
	SegmentID string `json:"segmentID"`
	MarkID    string `json:"markID"`
	Code      string `json:"code"`
	Comment   string `json:"comment"`
}

type ResolveReviewFindingPayload struct {
	ReviewRound    int    `json:"reviewRound"`
	SegmentID      string `json:"segmentID"`
	MarkID         string `json:"markID"`
	Code           string `json:"code"`
	CorrectionNote string `json:"correctionNote"`
}

type ReleasePayload struct {
	IssuedBy string `json:"issuedBy"`
}
