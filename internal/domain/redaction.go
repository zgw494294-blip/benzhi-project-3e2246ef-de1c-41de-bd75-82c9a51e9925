package domain

import (
	"sort"
	"time"
)

var validCategories = map[string]bool{"person": true, "place": true, "kinship": true, "ritual_knowledge": true}
var validActions = map[string]bool{"mask": true, "generalize": true, "retain": true}

func (c *CorpusCase) SetMarks(segmentID string, marks []RedactionMark, actor string, now time.Time) (Event, error) {
	return c.SetMarksForFindings(segmentID, marks, nil, actor, now)
}

func (c *CorpusCase) SetMarksForFindings(segmentID string, marks []RedactionMark, findingRefs []ReviewFindingRef, actor string, now time.Time) (Event, error) {
	if err := c.ensureMutable(); err != nil {
		return Event{}, err
	}
	if c.Status != StatusIntakeLocked && c.Status != StatusCorrection && !(c.Status == StatusRedacted && len(c.currentOpenFindings()) > 0) {
		return Event{}, Conflict("invalid_status", "当前状态不可更新敏感标注")
	}
	segment, ok := c.Segments[segmentID]
	if !ok {
		return Event{}, NotFound("转写片段", segmentID)
	}
	seen := map[string]bool{}
	for i := range marks {
		mark := &marks[i]
		if mark.MarkID == "" || seen[mark.MarkID] {
			return Event{}, Validation("invalid_mark_id", "标注 ID 为空或重复")
		}
		seen[mark.MarkID] = true
		mark.SegmentID = segmentID
		if !validCategories[mark.Category] {
			return Event{}, Validation("invalid_category", "标注 %s 的敏感类别无效", mark.MarkID)
		}
		if !validActions[mark.Action] {
			return Event{}, Validation("invalid_action", "标注 %s 的处理策略无效", mark.MarkID)
		}
		if mark.StartRune < 0 || mark.EndRune <= mark.StartRune || mark.EndRune > len([]rune(segment.SourceText)) {
			return Event{}, Validation("mark_out_of_bounds", "标注 %s 的 rune 区间越界", mark.MarkID)
		}
		if mark.Rationale == "" {
			return Event{}, Validation("rationale_required", "标注 %s 缺少处理依据", mark.MarkID)
		}
		if mark.Action == "generalize" && mark.ReplacementText == "" {
			return Event{}, Validation("replacement_required", "泛化标注必须提供替换文本")
		}
		if mark.ResolutionStatus != "resolved" && mark.ResolutionStatus != "unresolved" {
			return Event{}, Validation("invalid_resolution", "标注解决状态无效")
		}
	}
	ordered := append([]RedactionMark(nil), marks...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].StartRune != ordered[j].StartRune {
			return ordered[i].StartRune < ordered[j].StartRune
		}
		return ordered[i].EndRune < ordered[j].EndRune
	})
	for index := 1; index < len(ordered); index++ {
		if ordered[index].StartRune < ordered[index-1].EndRune {
			return Event{}, Validation("mark_overlap", "标注 %s 与标注 %s 的区间重叠", ordered[index].MarkID, ordered[index-1].MarkID)
		}
	}
	if err := c.markFindingsReady(segmentID, marks, findingRefs); err != nil {
		return Event{}, err
	}
	c.Marks[segmentID] = append([]RedactionMark(nil), marks...)
	delete(c.Redacted, segmentID)
	if c.Status == StatusRedacted {
		c.Status = StatusCorrection
	}
	return c.record("redaction.marks_set", actor, now, map[string]any{"segmentID": segmentID, "markCount": len(marks), "findingRefCount": len(findingRefs)}), nil
}

func (c *CorpusCase) ApplyRedacted(results map[string]RedactedSegment, actor string, now time.Time) (Event, error) {
	if c.Status != StatusIntakeLocked && c.Status != StatusCorrection {
		return Event{}, Conflict("invalid_status", "当前状态不可生成脱敏稿")
	}
	if len(results) != len(c.Segments) {
		return Event{}, Validation("redaction_incomplete", "脱敏结果未覆盖全部片段")
	}
	c.Redacted = results
	c.Status = StatusRedacted
	return c.record("redaction.generated", actor, now, map[string]any{"segmentCount": len(results)}), nil
}
