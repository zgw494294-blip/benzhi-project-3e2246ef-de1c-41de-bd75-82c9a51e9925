package domain

import (
	"errors"
	"testing"
	"time"
)

func TestIntakeRequiresConsentAndLocksSource(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	c, _, err := NewCase("case-1", "访谈", "zh", "测试", "owner", "archivist", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddRecording(Recording{RecordingID: "rec", Label: "录音", DurationMS: 10000}, "archivist", now); err != nil {
		t.Fatal(err)
	}
	segment := TranscriptSegment{SegmentID: "seg", RecordingID: "rec", SpeakerID: "speaker", StartMillis: 0, EndMillis: 1000, SourceText: "口述内容", Sequence: 1}
	if _, err := c.AddSegment(segment, "archivist", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.LockIntake("archivist", now); err == nil {
		t.Fatal("缺少授权时不应锁定")
	}
	consent := ConsentRecord{ConsentID: "consent", SpeakerID: "speaker", Scope: []string{"research_release"}, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour), EvidenceRef: "evidence"}
	if _, err := c.AddConsent(consent, "archivist", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.LockIntake("archivist", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusIntakeLocked {
		t.Fatalf("状态为 %s", c.Status)
	}
	segment.SegmentID = "late"
	if _, err := c.AddSegment(segment, "archivist", now); err == nil {
		t.Fatal("锁定后不应允许增加原始片段")
	}
}

func TestReviewReturnThenApproval(t *testing.T) {
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	c := &CorpusCase{CaseID: "c", Status: StatusRedacted, Revision: 7, Segments: map[string]TranscriptSegment{"s": {SegmentID: "s"}}, Redacted: map[string]RedactedSegment{"s": {SegmentID: "s"}}}
	if _, err := c.SubmitReview("a", now); err != nil {
		t.Fatal(err)
	}
	returned := ReviewDecision{DecisionID: "d1", ReviewerID: "r", SampledSegmentIDs: []string{"s"}, Findings: []ReviewFinding{{SegmentID: "s", Code: "exposed", Comment: "需修正"}}, Outcome: "returned"}
	if _, err := c.DecideReview(returned, "r", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusCorrection || len(c.OpenFindings) != 1 {
		t.Fatal("退回状态或意见未保存")
	}
	resolvedAt := now
	c.ReviewFindings[0].Status = "resolved"
	c.ReviewFindings[0].CorrectionNote = "测试中模拟已闭环"
	c.ReviewFindings[0].ResolvedBy = "a"
	c.ReviewFindings[0].ResolvedAt = &resolvedAt
	c.OpenFindings = nil
	c.Status = StatusUnderReview
	approved := ReviewDecision{DecisionID: "d2", ReviewerID: "r", SampledSegmentIDs: []string{"s"}, Outcome: "approved"}
	if _, err := c.DecideReview(approved, "r", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusReviewApproved || len(c.ReviewHistory) != 2 {
		t.Fatal("通过状态或历史不正确")
	}
}

func TestDraftMetadataUpdateIsControlled(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	c, _, err := NewCase("metadata-case", "原标题", "zh", "原背景", "owner-a", "archivist-a", now)
	if err != nil {
		t.Fatal(err)
	}
	title, owner := "新标题", "owner-b"
	event, err := c.UpdateMetadata(MetadataPatch{Title: &title, OwnerID: &owner}, "archivist-b", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if c.Revision != 2 || c.Title != title || c.OwnerID != owner || event.Type != "metadata.updated" {
		t.Fatalf("元数据修订结果不正确: %+v", c)
	}
	fields, ok := event.Facts["changedFields"].([]string)
	if !ok || len(fields) != 2 || fields[0] != "title" || fields[1] != "ownerID" {
		t.Fatalf("变更字段事实不正确: %+v", event.Facts)
	}
	if _, exists := event.Facts["collectionContext"]; exists {
		t.Fatal("事件不应复制采集背景原文")
	}
	beforeRevision := c.Revision
	c.Status = StatusIntakeLocked
	other := "锁定后标题"
	if _, err := c.UpdateMetadata(MetadataPatch{Title: &other}, "archivist-b", now); err == nil {
		t.Fatal("锁定后不应允许修订元数据")
	}
	if c.Title != title || c.Revision != beforeRevision {
		t.Fatal("状态冲突不应修改案卷")
	}
}

func TestBatchSegmentsCollectsIssuesAndCommitsAtomically(t *testing.T) {
	now := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	c, _, _ := NewCase("batch-case", "批量案卷", "zh", "", "owner", "archivist", now)
	if _, err := c.AddRecording(Recording{RecordingID: "rec", Label: "录音", DurationMS: 5000}, "archivist", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddSegment(TranscriptSegment{SegmentID: "existing", RecordingID: "rec", SpeakerID: "speaker", StartMillis: 0, EndMillis: 1000, SourceText: "既有", Sequence: 1}, "archivist", now); err != nil {
		t.Fatal(err)
	}
	revision := c.Revision
	segments := []TranscriptSegment{
		{SegmentID: "too-long", RecordingID: "rec", SpeakerID: "speaker", StartMillis: 1000, EndMillis: 6000, SourceText: "越界", Sequence: 2},
		{SegmentID: "same-sequence", RecordingID: "rec", SpeakerID: "speaker", StartMillis: 2000, EndMillis: 2500, SourceText: "冲突", Sequence: 1},
	}
	_, err := c.AddSegments(segments, "archivist", now)
	var business *BusinessError
	if !errors.As(err, &business) || business.Code != "batch_segments_invalid" {
		t.Fatalf("批量错误不正确: %v", err)
	}
	issues, ok := business.Details["issues"].([]BatchSegmentIssue)
	foundTiming, foundSequence := false, false
	for _, issue := range issues {
		foundTiming = foundTiming || (issue.Index == 0 && issue.Code == "invalid_timing")
		foundSequence = foundSequence || (issue.Index == 1 && issue.Code == "sequence_conflict")
	}
	if !ok || !foundTiming || !foundSequence {
		t.Fatalf("未同时返回可判定问题: %+v", business.Details)
	}
	if c.Revision != revision || len(c.Segments) != 1 {
		t.Fatal("失败批次留下了片段或修订")
	}
	segments[0].EndMillis = 1500
	segments[1].Sequence = 3
	if _, err := c.AddSegments(segments, "archivist", now); err != nil {
		t.Fatal(err)
	}
	if c.Revision != revision+1 || len(c.Segments) != 3 || c.Audit[len(c.Audit)-1].Type != "segment.batch_added" {
		t.Fatal("成功批次未按一次修订原子写入")
	}
}

func TestConsentCoverageClassifiesAndSortsSpeakers(t *testing.T) {
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	c, _, _ := NewCase("coverage-case", "授权案卷", "zh", "", "owner", "archivist", now)
	_, _ = c.AddRecording(Recording{RecordingID: "rec", Label: "录音", DurationMS: 10000}, "archivist", now)
	for index, speakerID := range []string{"speaker-c", "speaker-a", "speaker-b", "speaker-d"} {
		_, err := c.AddSegment(TranscriptSegment{
			SegmentID: speakerID, RecordingID: "rec", SpeakerID: speakerID,
			StartMillis: int64(index * 2000), EndMillis: int64(index*2000 + 1000), SourceText: "内容", Sequence: index + 1,
		}, "archivist", now)
		if err != nil {
			t.Fatal(err)
		}
	}
	consents := []ConsentRecord{
		{ConsentID: "a", SpeakerID: "speaker-a", Scope: []string{"research_release"}, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(60 * 24 * time.Hour), EvidenceRef: "e-a"},
		{ConsentID: "b", SpeakerID: "speaker-b", Scope: []string{"research_release"}, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(10 * 24 * time.Hour), EvidenceRef: "e-b"},
		{ConsentID: "c", SpeakerID: "speaker-c", Scope: []string{"research_release"}, ValidFrom: now.Add(-60 * 24 * time.Hour), ValidUntil: now.Add(-time.Hour), EvidenceRef: "e-c"},
		{ConsentID: "d", SpeakerID: "speaker-d", Scope: []string{"internal_processing"}, ValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(60 * 24 * time.Hour), EvidenceRef: "e-d"},
	}
	for _, consent := range consents {
		if _, err := c.AddConsent(consent, "archivist", now); err != nil {
			t.Fatal(err)
		}
	}
	coverage := c.ConsentCoverage(now)
	if coverage.Summary.TotalSpeakers != 4 || coverage.Summary.CoveredSpeakers != 2 || coverage.Summary.BlockedSpeakers != 2 || coverage.Summary.ExpiringSoonSpeakers != 1 {
		t.Fatalf("授权汇总不正确: %+v", coverage.Summary)
	}
	statuses := []string{"valid", "valid", "expired", "terms_blocked"}
	for index, detail := range coverage.Speakers {
		if detail.SpeakerID != []string{"speaker-a", "speaker-b", "speaker-c", "speaker-d"}[index] || detail.Status != statuses[index] {
			t.Fatalf("授权明细排序或分类不正确: %+v", coverage.Speakers)
		}
	}
	var business *BusinessError
	if _, err := c.LockIntake("archivist", now); err == nil {
		t.Fatal("存在已过期授权时不应锁定")
	} else if !errors.As(err, &business) || business.Code != coverage.Speakers[2].BlockerCode {
		t.Fatalf("锁定错误码与覆盖分类不一致: %v", err)
	}
}

func TestReviewFindingsResolveOneByOne(t *testing.T) {
	now := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	c := &CorpusCase{
		CaseID: "review-case", Status: StatusUnderReview,
		Segments: map[string]TranscriptSegment{"s": {SegmentID: "s", SourceText: "甲乙"}},
		Marks:    map[string][]RedactionMark{}, Redacted: map[string]RedactedSegment{},
	}
	decision := ReviewDecision{DecisionID: "returned", ReviewerID: "reviewer", Outcome: "returned", Findings: []ReviewFinding{
		{SegmentID: "s", MarkID: "m1", Code: "person_exposed", Comment: "处理甲"},
		{SegmentID: "s", MarkID: "m2", Code: "place_exposed", Comment: "处理乙"},
	}}
	if _, err := c.DecideReview(decision, "reviewer", now); err != nil {
		t.Fatal(err)
	}
	marks := []RedactionMark{
		{MarkID: "m1", Category: "person", StartRune: 0, EndRune: 1, Action: "mask", Rationale: "复核意见", ResolutionStatus: "resolved"},
		{MarkID: "m2", Category: "place", StartRune: 1, EndRune: 2, Action: "mask", Rationale: "复核意见", ResolutionStatus: "resolved"},
	}
	first := ReviewFindingRef{ReviewRound: 1, SegmentID: "s", MarkID: "m1", Code: "person_exposed"}
	if _, err := c.SetMarksForFindings("s", marks, []ReviewFindingRef{first}, "archivist", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolveReviewFinding(first, "已遮蔽人名", "archivist", now); err != nil {
		t.Fatal(err)
	}
	if len(c.OpenFindings) != 1 || c.ReviewFindings[0].Status != "resolved" || c.ReviewFindings[1].Status != "open" {
		t.Fatalf("逐项状态不正确: %+v", c.ReviewFindings)
	}
	if _, err := c.SubmitReview("archivist", now); err == nil {
		t.Fatal("尚有一条开放意见时不应提交")
	}
	if _, err := c.ResolveReviewFinding(first, "覆盖首次说明", "archivist", now); err == nil {
		t.Fatal("重复解决应冲突")
	}
	if c.ReviewFindings[0].CorrectionNote != "已遮蔽人名" {
		t.Fatal("重复解决覆盖了首次处置事实")
	}
	second := ReviewFindingRef{ReviewRound: 1, SegmentID: "s", MarkID: "m2", Code: "place_exposed"}
	marks[1].Rationale = "按第二条复核意见完成处理"
	if _, err := c.SetMarksForFindings("s", marks, []ReviewFindingRef{second}, "archivist", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ResolveReviewFinding(second, "已遮蔽地点", "archivist", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ApplyRedacted(map[string]RedactedSegment{"s": {SegmentID: "s", Text: "[已处理]"}}, "archivist", now); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SubmitReview("archivist", now); err != nil {
		t.Fatal(err)
	}
}
