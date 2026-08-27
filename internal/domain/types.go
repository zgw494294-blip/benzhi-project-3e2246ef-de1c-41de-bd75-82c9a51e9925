package domain

import "time"

type CaseStatus string

const (
	StatusDraft          CaseStatus = "draft"
	StatusIntakeLocked   CaseStatus = "intake_locked"
	StatusRedacted       CaseStatus = "redacted"
	StatusUnderReview    CaseStatus = "under_review"
	StatusCorrection     CaseStatus = "correction_required"
	StatusReviewApproved CaseStatus = "review_approved"
	StatusReleased       CaseStatus = "released"
)

type Recording struct {
	RecordingID string `json:"recordingID"`
	Label       string `json:"label"`
	DurationMS  int64  `json:"durationMillis"`
	EvidenceRef string `json:"evidenceRef"`
}

type ConsentRecord struct {
	ConsentID    string    `json:"consentID"`
	CaseID       string    `json:"caseID"`
	SpeakerID    string    `json:"speakerID"`
	Scope        []string  `json:"scope"`
	Restrictions []string  `json:"restrictions,omitempty"`
	ValidFrom    time.Time `json:"validFrom"`
	ValidUntil   time.Time `json:"validUntil"`
	EvidenceRef  string    `json:"evidenceRef"`
	RecordedAt   time.Time `json:"recordedAt"`
}

type TranscriptSegment struct {
	SegmentID   string `json:"segmentID"`
	CaseID      string `json:"caseID"`
	RecordingID string `json:"recordingID"`
	SpeakerID   string `json:"speakerID"`
	StartMillis int64  `json:"startMillis"`
	EndMillis   int64  `json:"endMillis"`
	SourceText  string `json:"sourceText"`
	Sequence    int    `json:"sequence"`
}

type RedactionMark struct {
	MarkID           string `json:"markID"`
	SegmentID        string `json:"segmentID"`
	Category         string `json:"category"`
	StartRune        int    `json:"startRune"`
	EndRune          int    `json:"endRune"`
	Action           string `json:"action"`
	ReplacementText  string `json:"replacementText,omitempty"`
	Rationale        string `json:"rationale"`
	ResolutionStatus string `json:"resolutionStatus"`
}

type TextMapping struct {
	MarkID          string `json:"markID"`
	SourceStartRune int    `json:"sourceStartRune"`
	SourceEndRune   int    `json:"sourceEndRune"`
	Replacement     string `json:"replacement"`
}

type RedactedSegment struct {
	SegmentID    string        `json:"segmentID"`
	Text         string        `json:"text"`
	Mappings     []TextMapping `json:"mappings"`
	SourceDigest string        `json:"sourceDigest"`
}

type ReviewFinding struct {
	ReviewRound     int        `json:"reviewRound,omitempty"`
	SegmentID       string     `json:"segmentID"`
	MarkID          string     `json:"markID,omitempty"`
	Code            string     `json:"code"`
	Comment         string     `json:"comment"`
	Status          string     `json:"status,omitempty"`
	ResolutionReady bool       `json:"resolutionReady,omitempty"`
	CorrectionNote  string     `json:"correctionNote,omitempty"`
	ResolvedBy      string     `json:"resolvedBy,omitempty"`
	ResolvedAt      *time.Time `json:"resolvedAt,omitempty"`
}

type ReviewFindingRef struct {
	ReviewRound int    `json:"reviewRound"`
	SegmentID   string `json:"segmentID"`
	MarkID      string `json:"markID,omitempty"`
	Code        string `json:"code"`
}

type ConsentCoverageDetail struct {
	SpeakerID                string   `json:"speakerID"`
	Status                   string   `json:"status"`
	BlockerCode              string   `json:"blockerCode,omitempty"`
	ConsentID                string   `json:"consentID,omitempty"`
	EvidenceRef              string   `json:"evidenceRef,omitempty"`
	AffectedSegmentIDs       []string `json:"affectedSegmentIDs"`
	RemainingValiditySeconds int64    `json:"remainingValiditySeconds"`
	ExpiringSoon             bool     `json:"expiringSoon"`
}

type ConsentCoverageSummary struct {
	TotalSpeakers        int `json:"totalSpeakers"`
	CoveredSpeakers      int `json:"coveredSpeakers"`
	BlockedSpeakers      int `json:"blockedSpeakers"`
	ExpiringSoonSpeakers int `json:"expiringSoonSpeakers"`
}

type ConsentCoverage struct {
	AsOf       time.Time               `json:"asOf"`
	WindowDays int                     `json:"riskWindowDays"`
	Summary    ConsentCoverageSummary  `json:"summary"`
	Speakers   []ConsentCoverageDetail `json:"speakers"`
}

type ReviewDecision struct {
	DecisionID        string          `json:"decisionID"`
	CaseID            string          `json:"caseID"`
	ReviewRound       int             `json:"reviewRound"`
	ReviewerID        string          `json:"reviewerID"`
	SampledSegmentIDs []string        `json:"sampledSegmentIDs"`
	Findings          []ReviewFinding `json:"findings"`
	Outcome           string          `json:"outcome"`
	DecidedAt         time.Time       `json:"decidedAt"`
}

type FrozenItem struct {
	SegmentID    string `json:"segmentID"`
	RecordingID  string `json:"recordingID"`
	Sequence     int    `json:"sequence"`
	Text         string `json:"text"`
	SourceDigest string `json:"sourceDigest"`
}

type FrozenManifest struct {
	CaseID         string       `json:"caseID"`
	FrozenRevision uint64       `json:"frozenRevision"`
	Items          []FrozenItem `json:"items"`
	Digest         string       `json:"digest"`
	FrozenAt       time.Time    `json:"frozenAt"`
}

type ReleaseCredential struct {
	CredentialID       string    `json:"credentialID"`
	CaseID             string    `json:"caseID"`
	FrozenRevision     uint64    `json:"frozenRevision"`
	ManifestDigest     string    `json:"manifestDigest"`
	IssuedBy           string    `json:"issuedBy"`
	IssuedAt           time.Time `json:"issuedAt"`
	VerificationStatus string    `json:"verificationStatus"`
}
