package redaction

import (
	"testing"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

func TestGenerateUsesRuneOffsets(t *testing.T) {
	segment := domain.TranscriptSegment{SegmentID: "s", SourceText: "甲😀阿明乙"}
	marks := []domain.RedactionMark{{MarkID: "m", SegmentID: "s", Category: "person", StartRune: 2, EndRune: 4, Action: "mask", Rationale: "身份", ResolutionStatus: "resolved"}}
	result, issues := Generate(segment, marks)
	if len(issues) != 0 {
		t.Fatalf("意外问题: %+v", issues)
	}
	if result.Text != "甲😀[已遮蔽:person]乙" {
		t.Fatalf("结果为 %q", result.Text)
	}
	if len(result.Mappings) != 1 || result.Mappings[0].SourceStartRune != 2 {
		t.Fatal("映射不正确")
	}
}

func TestReleasePreviewAndReleaseUseSamePreflight(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	segment := domain.TranscriptSegment{SegmentID: "s", RecordingID: "r", SpeakerID: "speaker", Sequence: 1, SourceText: "原文"}
	redacted, issues := Generate(segment, nil)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	c := &domain.CorpusCase{
		CaseID: "release-case", Status: domain.StatusReviewApproved,
		Segments: map[string]domain.TranscriptSegment{"s": segment}, Redacted: map[string]domain.RedactedSegment{"s": redacted},
		Consents: map[string]domain.ConsentRecord{"consent": {
			ConsentID: "consent", SpeakerID: "speaker", Scope: []string{"research_release"},
			ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), EvidenceRef: "evidence",
		}},
	}
	engine := New()
	preview, err := engine.ReleasePreview(c, now)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Available || preview.Status != "ready" || preview.ItemCount != 1 || len(preview.Blockers) != 0 {
		t.Fatalf("放行预检结果不正确: %+v", preview)
	}
	manifest, err := engine.BuildReleaseManifest(c, now)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Digest != preview.ManifestDigest {
		t.Fatal("预检摘要与放行摘要不一致")
	}

	expiredPreview, err := engine.ReleasePreview(c, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if expiredPreview.Status != "blocked" || len(expiredPreview.Blockers) != 1 || expiredPreview.Blockers[0].Code != "consent_expired" {
		t.Fatalf("授权到期未被定位: %+v", expiredPreview)
	}
	if _, err := engine.BuildReleaseManifest(c, now.Add(2*time.Hour)); err == nil {
		t.Fatal("授权到期后不应允许构建放行清单")
	}

	c.Redacted["s"] = domain.RedactedSegment{SegmentID: "s", Text: "脱敏稿", SourceDigest: "tampered"}
	digestPreview, err := engine.ReleasePreview(c, now)
	if err != nil {
		t.Fatal(err)
	}
	if digestPreview.Status != "blocked" || len(digestPreview.Blockers) != 1 || digestPreview.Blockers[0].Code != "source_digest_mismatch" {
		t.Fatalf("来源摘要不一致未被定位: %+v", digestPreview)
	}
}

func TestOverlapAndUnresolvedAreBlockers(t *testing.T) {
	segment := domain.TranscriptSegment{SegmentID: "s", SourceText: "一二三四五"}
	marks := []domain.RedactionMark{
		{MarkID: "a", StartRune: 0, EndRune: 3, ResolutionStatus: "resolved"},
		{MarkID: "b", StartRune: 2, EndRune: 4, ResolutionStatus: "unresolved"},
	}
	_, issues := Normalize(segment, marks)
	if len(issues) != 2 {
		t.Fatalf("期望两个问题，得到 %+v", issues)
	}
}

func TestManifestDigestIsStable(t *testing.T) {
	c := &domain.CorpusCase{CaseID: "c", Segments: map[string]domain.TranscriptSegment{
		"b": {SegmentID: "b", RecordingID: "r", Sequence: 2}, "a": {SegmentID: "a", RecordingID: "r", Sequence: 1},
	}, Redacted: map[string]domain.RedactedSegment{
		"a": {SegmentID: "a", Text: "甲", SourceDigest: "1"}, "b": {SegmentID: "b", Text: "乙", SourceDigest: "2"},
	}}
	first, err := New().BuildManifest(c, c.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New().BuildManifest(c, c.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest || !VerifyManifest(first) {
		t.Fatal("摘要不稳定或不可验证")
	}
}
