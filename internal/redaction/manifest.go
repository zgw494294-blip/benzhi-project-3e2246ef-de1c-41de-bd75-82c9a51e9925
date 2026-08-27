package redaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

type digestDocument struct {
	CaseID string              `json:"caseID"`
	Items  []domain.FrozenItem `json:"items"`
}

func (e *Engine) BuildManifest(c *domain.CorpusCase, now time.Time) (domain.FrozenManifest, error) {
	manifest, _, err := e.preflight(c, now)
	if err != nil {
		return domain.FrozenManifest{}, err
	}
	return manifest, nil
}

func (e *Engine) BuildReleaseManifest(c *domain.CorpusCase, now time.Time) (domain.FrozenManifest, error) {
	if c.Status != domain.StatusReviewApproved {
		return domain.FrozenManifest{}, domain.Conflict("invalid_status", "仅复核通过的案卷可执行放行核验")
	}
	manifest, issues, err := e.preflight(c, now)
	if err != nil {
		return domain.FrozenManifest{}, err
	}
	if len(issues) > 0 {
		business := domain.NewError(domain.KindValidation, "release_blocked", "放行前核验存在阻断项")
		business.Details = map[string]any{"issues": issues}
		return domain.FrozenManifest{}, business
	}
	return manifest, nil
}

func (e *Engine) ReleasePreview(c *domain.CorpusCase, now time.Time) (ReleasePreview, error) {
	if c.Status != domain.StatusReviewApproved {
		return ReleasePreview{
			Available: false, Status: "unavailable", Items: []domain.FrozenItem{}, Blockers: []Issue{{
				Code: "release_preview_status_conflict", Message: "仅复核通过的案卷可执行放行预检",
			}},
		}, nil
	}
	manifest, issues, err := e.preflight(c, now)
	if err != nil {
		return ReleasePreview{}, err
	}
	status := "ready"
	if len(issues) > 0 {
		status = "blocked"
	}
	return ReleasePreview{
		Available: true, Status: status, Items: manifest.Items, ItemCount: len(manifest.Items),
		ManifestDigest: manifest.Digest, Blockers: issues,
	}, nil
}

func (e *Engine) preflight(c *domain.CorpusCase, now time.Time) (domain.FrozenManifest, []Issue, error) {
	items := make([]domain.FrozenItem, 0, len(c.Segments))
	issues := make([]Issue, 0)
	coverageBySpeaker := make(map[string]domain.ConsentCoverageDetail)
	for _, detail := range e.coverageDetails(c, now) {
		coverageBySpeaker[detail.SpeakerID] = detail
	}
	for _, finding := range c.OpenFindings {
		if finding.Status == "" || finding.Status == "open" {
			issues = append(issues, Issue{Code: "review_finding_open", ReviewRound: finding.ReviewRound, SegmentID: finding.SegmentID, MarkID: finding.MarkID, Message: "复核意见尚未闭环"})
		}
	}
	for _, segmentID := range e.sortedSegmentIDs(c) {
		segment := c.Segments[segmentID]
		if detail := coverageBySpeaker[segment.SpeakerID]; detail.Status != "valid" {
			issues = append(issues, Issue{Code: detail.BlockerCode, SegmentID: segmentID, ConsentID: detail.ConsentID, Message: "说话人当前授权不能覆盖研究开放"})
		}
		redacted, ok := c.Redacted[segmentID]
		if !ok {
			issues = append(issues, Issue{Code: "redaction_incomplete", SegmentID: segmentID, Message: "片段缺少脱敏稿"})
			continue
		}
		expectedSourceDigest := sourceDigest(segment.SourceText)
		if redacted.SourceDigest != expectedSourceDigest {
			issues = append(issues, Issue{Code: "source_digest_mismatch", SegmentID: segmentID, Message: "脱敏稿来源摘要与当前原文不一致"})
		}
		items = append(items, domain.FrozenItem{
			SegmentID: segmentID, RecordingID: segment.RecordingID, Sequence: segment.Sequence,
			Text: redacted.Text, SourceDigest: redacted.SourceDigest,
		})
	}
	document, err := json.Marshal(digestDocument{CaseID: c.CaseID, Items: items})
	if err != nil {
		return domain.FrozenManifest{}, nil, err
	}
	digest := sha256.Sum256(document)
	return domain.FrozenManifest{
		CaseID: c.CaseID, Items: items, Digest: hex.EncodeToString(digest[:]), FrozenAt: now.UTC(),
	}, issues, nil
}

func sourceDigest(sourceText string) string {
	digest := sha256.Sum256([]byte(sourceText))
	return hex.EncodeToString(digest[:])
}

func VerifyManifest(manifest domain.FrozenManifest) bool {
	document, err := json.Marshal(digestDocument{CaseID: manifest.CaseID, Items: manifest.Items})
	if err != nil {
		return false
	}
	digest := sha256.Sum256(document)
	return hex.EncodeToString(digest[:]) == manifest.Digest
}
