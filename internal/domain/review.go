package domain

import (
	"sort"
	"strings"
	"time"
)

func (c *CorpusCase) SubmitReview(actor string, now time.Time) (Event, error) {
	open := c.currentOpenFindings()
	if len(open) > 0 {
		business := NewError(KindValidation, "review_findings_open", "仍有复核意见尚未逐项闭环")
		refs := make([]ReviewFindingRef, 0, len(open))
		for _, finding := range open {
			refs = append(refs, finding.ref())
		}
		business.Details = map[string]any{"findings": refs}
		return Event{}, business
	}
	if c.Status != StatusRedacted {
		return Event{}, Conflict("invalid_status", "仅完整脱敏稿可提交隐私复核")
	}
	if len(c.Redacted) != len(c.Segments) {
		return Event{}, Validation("redaction_incomplete", "脱敏稿未覆盖全部片段")
	}
	c.Status = StatusUnderReview
	round := len(c.ReviewHistory) + 1
	return c.record("review.submitted", actor, now, map[string]any{"reviewRound": round}), nil
}

func (c *CorpusCase) DecideReview(decision ReviewDecision, actor string, now time.Time) (Event, error) {
	if c.Status != StatusUnderReview {
		return Event{}, Conflict("invalid_status", "案卷当前不在隐私复核中")
	}
	if decision.DecisionID == "" || decision.ReviewerID == "" {
		return Event{}, Validation("invalid_decision", "复核决定 ID 和复核员不能为空")
	}
	if decision.Outcome != "approved" && decision.Outcome != "returned" {
		return Event{}, Validation("invalid_outcome", "复核结论必须为 approved 或 returned")
	}
	if decision.Outcome == "returned" && len(decision.Findings) == 0 {
		return Event{}, Validation("findings_required", "退回修正必须包含逐片段意见")
	}
	if decision.Outcome == "approved" && len(c.currentOpenFindings()) > 0 {
		return Event{}, Conflict("review_findings_open", "仍有复核意见尚未逐项闭环，不能通过复核")
	}
	for _, prior := range c.ReviewHistory {
		if prior.DecisionID == decision.DecisionID {
			return Event{}, Conflict("decision_exists", "复核决定 ID 已存在")
		}
	}
	for _, segmentID := range decision.SampledSegmentIDs {
		if _, ok := c.Segments[segmentID]; !ok {
			return Event{}, Validation("sample_unknown", "抽样片段 %s 不存在", segmentID)
		}
	}
	for _, finding := range decision.Findings {
		if _, ok := c.Segments[finding.SegmentID]; !ok {
			return Event{}, Validation("finding_segment_unknown", "意见引用未知片段")
		}
		if finding.Code == "" || finding.Comment == "" {
			return Event{}, Validation("invalid_finding", "意见代码和说明不能为空")
		}
	}
	decision.CaseID = c.CaseID
	decision.ReviewRound = len(c.ReviewHistory) + 1
	decision.DecidedAt = now.UTC()
	seenFindings := map[string]bool{}
	for index := range decision.Findings {
		finding := &decision.Findings[index]
		finding.ReviewRound = decision.ReviewRound
		key := findingKey(finding.ref())
		if seenFindings[key] {
			return Event{}, Validation("finding_duplicate", "同一复核决定包含重复意见定位键")
		}
		seenFindings[key] = true
	}
	c.ReviewHistory = append(c.ReviewHistory, decision)
	if decision.Outcome == "returned" {
		c.Status = StatusCorrection
		for _, finding := range decision.Findings {
			tracked := finding
			tracked.Status = "open"
			c.ReviewFindings = append(c.ReviewFindings, tracked)
		}
		c.sortReviewFindings()
		c.refreshOpenFindings()
	} else {
		c.Status = StatusReviewApproved
		c.OpenFindings = nil
	}
	return c.record("review."+decision.Outcome, actor, now, map[string]any{"decisionID": decision.DecisionID, "reviewRound": decision.ReviewRound}), nil
}

func (c *CorpusCase) ResolveReviewFinding(ref ReviewFindingRef, correctionNote, actor string, now time.Time) (Event, error) {
	if c.Status != StatusCorrection {
		return Event{}, Conflict("invalid_status", "仅退回修正状态可解决复核意见")
	}
	if ref.ReviewRound <= 0 || strings.TrimSpace(ref.SegmentID) == "" || strings.TrimSpace(ref.Code) == "" {
		return Event{}, Validation("invalid_finding_reference", "复核轮次、片段 ID 和意见代码不能为空")
	}
	if strings.TrimSpace(correctionNote) == "" {
		return Event{}, Validation("correction_note_required", "修正说明不能为空")
	}
	index := c.findReviewFinding(ref)
	if index < 0 {
		return Event{}, NotFound("复核待处理项", findingKey(ref))
	}
	finding := &c.ReviewFindings[index]
	if finding.Status == "resolved" {
		return Event{}, Conflict("finding_already_resolved", "复核意见已经闭环")
	}
	if _, ok := c.Segments[finding.SegmentID]; !ok {
		return Event{}, Conflict("finding_segment_missing", "复核意见关联片段已不存在")
	}
	if finding.MarkID != "" && !c.hasMark(finding.SegmentID, finding.MarkID) {
		return Event{}, Conflict("finding_mark_missing", "复核意见关联标注已不存在")
	}
	if !finding.ResolutionReady {
		return Event{}, Conflict("finding_correction_not_recorded", "必须先通过 set_marks 显式引用并处理该意见")
	}
	resolvedAt := now.UTC()
	finding.Status = "resolved"
	finding.CorrectionNote = correctionNote
	finding.ResolvedBy = actor
	finding.ResolvedAt = &resolvedAt
	c.refreshOpenFindings()
	return c.record("review.finding_resolved", actor, now, map[string]any{
		"reviewRound": ref.ReviewRound, "segmentID": ref.SegmentID, "markID": ref.MarkID, "code": ref.Code,
	}), nil
}

func (c *CorpusCase) markFindingsReady(segmentID string, marks []RedactionMark, refs []ReviewFindingRef) error {
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.SegmentID != segmentID {
			return Validation("finding_segment_mismatch", "set_marks 只能引用当前片段的复核意见")
		}
		key := findingKey(ref)
		if seen[key] {
			return Validation("finding_reference_duplicate", "set_marks 包含重复复核意见引用")
		}
		seen[key] = true
		index := c.findReviewFinding(ref)
		if index < 0 {
			return NotFound("复核待处理项", key)
		}
		finding := c.ReviewFindings[index]
		if finding.Status != "open" {
			return Conflict("finding_already_resolved", "复核意见已经闭环")
		}
		if ref.MarkID != "" && !marksContain(marks, ref.MarkID) {
			return Validation("finding_mark_missing", "更新后的标注中缺少意见关联 markID")
		}
		if !c.findingCorrectionApplied(segmentID, ref.MarkID, marks) {
			return Conflict("finding_correction_not_applied", "set_marks 未对该意见关联标注或片段产生实际修正")
		}
	}
	for _, ref := range refs {
		c.ReviewFindings[c.findReviewFinding(ref)].ResolutionReady = true
	}
	c.refreshOpenFindings()
	return nil
}

func (c *CorpusCase) findingCorrectionApplied(segmentID, markID string, marks []RedactionMark) bool {
	if markID != "" {
		var before, after *RedactionMark
		for index := range c.Marks[segmentID] {
			if c.Marks[segmentID][index].MarkID == markID {
				before = &c.Marks[segmentID][index]
				break
			}
		}
		for index := range marks {
			if marks[index].MarkID == markID {
				after = &marks[index]
				break
			}
		}
		return after != nil && (before == nil || !sameMark(*before, *after))
	}
	before := c.Marks[segmentID]
	if len(before) != len(marks) {
		return true
	}
	for _, mark := range marks {
		matched := false
		for _, existing := range before {
			if existing.MarkID == mark.MarkID && sameMark(existing, mark) {
				matched = true
				break
			}
		}
		if !matched {
			return true
		}
	}
	return false
}

func sameMark(left, right RedactionMark) bool {
	return left.MarkID == right.MarkID && left.Category == right.Category && left.StartRune == right.StartRune &&
		left.EndRune == right.EndRune && left.Action == right.Action && left.ReplacementText == right.ReplacementText &&
		left.Rationale == right.Rationale && left.ResolutionStatus == right.ResolutionStatus
}

func (c *CorpusCase) currentOpenFindings() []ReviewFinding {
	if len(c.ReviewFindings) == 0 {
		return append([]ReviewFinding(nil), c.OpenFindings...)
	}
	result := make([]ReviewFinding, 0)
	for _, finding := range c.ReviewFindings {
		if finding.Status == "open" {
			result = append(result, finding)
		}
	}
	return result
}

func (c *CorpusCase) refreshOpenFindings() {
	c.OpenFindings = c.currentOpenFindings()
}

func (c *CorpusCase) findReviewFinding(ref ReviewFindingRef) int {
	wanted := findingKey(ref)
	for index := range c.ReviewFindings {
		if findingKey(c.ReviewFindings[index].ref()) == wanted {
			return index
		}
	}
	return -1
}

func (c *CorpusCase) hasMark(segmentID, markID string) bool {
	return marksContain(c.Marks[segmentID], markID)
}

func marksContain(marks []RedactionMark, markID string) bool {
	for _, mark := range marks {
		if mark.MarkID == markID {
			return true
		}
	}
	return false
}

func (f ReviewFinding) ref() ReviewFindingRef {
	return ReviewFindingRef{ReviewRound: f.ReviewRound, SegmentID: f.SegmentID, MarkID: f.MarkID, Code: f.Code}
}

func findingKey(ref ReviewFindingRef) string {
	return formatRevision(uint64(ref.ReviewRound)) + "\x00" + ref.SegmentID + "\x00" + ref.MarkID + "\x00" + ref.Code
}

func (c *CorpusCase) sortReviewFindings() {
	sort.Slice(c.ReviewFindings, func(i, j int) bool {
		left, right := c.ReviewFindings[i], c.ReviewFindings[j]
		if left.ReviewRound != right.ReviewRound {
			return left.ReviewRound < right.ReviewRound
		}
		if left.SegmentID != right.SegmentID {
			return left.SegmentID < right.SegmentID
		}
		if left.MarkID != right.MarkID {
			return left.MarkID < right.MarkID
		}
		return left.Code < right.Code
	})
}
