package domain

import (
	"fmt"
	"sort"
	"strings"
)

var knownStatuses = map[CaseStatus]bool{
	StatusDraft: true, StatusIntakeLocked: true, StatusRedacted: true,
	StatusUnderReview: true, StatusCorrection: true, StatusReviewApproved: true, StatusReleased: true,
}

// ValidateProjection verifies the complete aggregate before it becomes a durable projection.
func (c *CorpusCase) ValidateProjection() error {
	if c == nil {
		return fmt.Errorf("案卷投影为空")
	}
	if c.CaseID == "" || c.Title == "" || c.LanguageCode == "" || c.OwnerID == "" {
		return fmt.Errorf("案卷基础字段不完整")
	}
	if !knownStatuses[c.Status] {
		return fmt.Errorf("未知案卷状态 %s", c.Status)
	}
	if c.Revision == 0 || len(c.Audit) == 0 {
		return fmt.Errorf("案卷缺少修订或审计事实")
	}
	if err := validateAudit(c); err != nil {
		return err
	}
	if err := validateRecordings(c); err != nil {
		return err
	}
	if err := validateConsents(c); err != nil {
		return err
	}
	if err := validateSegments(c); err != nil {
		return err
	}
	if err := validateMarks(c); err != nil {
		return err
	}
	if err := validateReviews(c); err != nil {
		return err
	}
	if c.Status == StatusReleased {
		if c.Manifest == nil || c.Credential == nil {
			return fmt.Errorf("已放行案卷缺少冻结清单或凭据")
		}
		if c.Manifest.CaseID != c.CaseID || c.Credential.CaseID != c.CaseID {
			return fmt.Errorf("冻结材料归属错误")
		}
		if c.Manifest.Digest != c.Credential.ManifestDigest {
			return fmt.Errorf("凭据摘要与冻结清单不一致")
		}
		if c.Manifest.FrozenRevision != c.Revision || c.Credential.FrozenRevision != c.Revision {
			return fmt.Errorf("冻结修订号与案卷不一致")
		}
	} else if c.Manifest != nil || c.Credential != nil {
		return fmt.Errorf("未放行案卷不能包含冻结清单或凭据")
	}
	return nil
}

func validateAudit(c *CorpusCase) error {
	previous := uint64(0)
	for _, fact := range c.Audit {
		if fact.Revision != previous+1 {
			return fmt.Errorf("审计事实修订号不连续")
		}
		if fact.EventID == "" || fact.Type == "" || fact.ActorID == "" || fact.OccurredAt.IsZero() {
			return fmt.Errorf("审计事实字段不完整")
		}
		previous = fact.Revision
	}
	if previous != c.Revision {
		return fmt.Errorf("审计事实未覆盖当前修订号")
	}
	return nil
}

func validateRecordings(c *CorpusCase) error {
	for id, recording := range c.Recordings {
		if id == "" || id != recording.RecordingID || recording.DurationMS <= 0 {
			return fmt.Errorf("录音投影 %s 无效", id)
		}
	}
	return nil
}

func validateConsents(c *CorpusCase) error {
	for id, consent := range c.Consents {
		if id == "" || id != consent.ConsentID || consent.CaseID != c.CaseID {
			return fmt.Errorf("授权投影 %s 归属无效", id)
		}
		if consent.SpeakerID == "" || consent.EvidenceRef == "" || !consent.ValidUntil.After(consent.ValidFrom) {
			return fmt.Errorf("授权投影 %s 字段无效", id)
		}
		if len(consent.Scope) == 0 {
			return fmt.Errorf("授权投影 %s 缺少授权范围", id)
		}
	}
	return nil
}

func validateSegments(c *CorpusCase) error {
	type position struct {
		recording string
		sequence  int
	}
	seen := map[position]string{}
	byRecording := map[string][]TranscriptSegment{}
	for id, segment := range c.Segments {
		if id == "" || id != segment.SegmentID || segment.CaseID != c.CaseID {
			return fmt.Errorf("片段投影 %s 归属无效", id)
		}
		recording, ok := c.Recordings[segment.RecordingID]
		if !ok {
			return fmt.Errorf("片段 %s 引用未知录音", id)
		}
		if segment.StartMillis < 0 || segment.EndMillis <= segment.StartMillis || segment.EndMillis > recording.DurationMS {
			return fmt.Errorf("片段 %s 时间位置无效", id)
		}
		if segment.Sequence <= 0 || strings.TrimSpace(segment.SpeakerID) == "" || strings.TrimSpace(segment.SourceText) == "" {
			return fmt.Errorf("片段 %s 序号、说话人或原文无效", id)
		}
		key := position{recording: segment.RecordingID, sequence: segment.Sequence}
		if prior, exists := seen[key]; exists {
			return fmt.Errorf("片段 %s 与 %s 序号冲突", id, prior)
		}
		seen[key] = id
		byRecording[segment.RecordingID] = append(byRecording[segment.RecordingID], segment)
	}
	for recordingID, segments := range byRecording {
		sort.Slice(segments, func(i, j int) bool {
			if segments[i].StartMillis != segments[j].StartMillis {
				return segments[i].StartMillis < segments[j].StartMillis
			}
			return segments[i].SegmentID < segments[j].SegmentID
		})
		for index := 1; index < len(segments); index++ {
			if segments[index].StartMillis < segments[index-1].EndMillis {
				return fmt.Errorf("录音 %s 的片段 %s 与 %s 时间区间重叠", recordingID, segments[index-1].SegmentID, segments[index].SegmentID)
			}
		}
	}
	return nil
}

func validateMarks(c *CorpusCase) error {
	for segmentID, marks := range c.Marks {
		segment, ok := c.Segments[segmentID]
		if !ok {
			return fmt.Errorf("标注引用未知片段 %s", segmentID)
		}
		ordered := append([]RedactionMark(nil), marks...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartRune < ordered[j].StartRune })
		seen := map[string]bool{}
		for index, mark := range ordered {
			if mark.MarkID == "" || seen[mark.MarkID] || mark.SegmentID != segmentID {
				return fmt.Errorf("片段 %s 的标注 ID 或归属无效", segmentID)
			}
			seen[mark.MarkID] = true
			if mark.StartRune < 0 || mark.EndRune <= mark.StartRune || mark.EndRune > len([]rune(segment.SourceText)) {
				return fmt.Errorf("标注 %s 越界", mark.MarkID)
			}
			if index > 0 && mark.StartRune < ordered[index-1].EndRune {
				return fmt.Errorf("标注 %s 与前一标注重叠", mark.MarkID)
			}
		}
	}
	return nil
}

func validateReviews(c *CorpusCase) error {
	seen := map[string]bool{}
	returnedKeys := map[string]bool{}
	for index, decision := range c.ReviewHistory {
		if decision.DecisionID == "" || seen[decision.DecisionID] || decision.ReviewRound != index+1 {
			return fmt.Errorf("复核决定序号或 ID 无效")
		}
		seen[decision.DecisionID] = true
		for _, segmentID := range decision.SampledSegmentIDs {
			if _, ok := c.Segments[segmentID]; !ok {
				return fmt.Errorf("复核抽样引用未知片段")
			}
		}
		for _, finding := range decision.Findings {
			if finding.SegmentID == "" || finding.Code == "" || finding.Comment == "" {
				return fmt.Errorf("复核意见字段不完整")
			}
			if finding.ReviewRound != 0 && finding.ReviewRound != decision.ReviewRound {
				return fmt.Errorf("复核意见轮次与决定不一致")
			}
			if decision.Outcome == "returned" {
				ref := ReviewFindingRef{ReviewRound: decision.ReviewRound, SegmentID: finding.SegmentID, MarkID: finding.MarkID, Code: finding.Code}
				returnedKeys[findingKey(ref)] = true
			}
		}
	}
	trackedKeys := map[string]bool{}
	openKeys := map[string]bool{}
	for _, finding := range c.ReviewFindings {
		key := findingKey(finding.ref())
		if !returnedKeys[key] || trackedKeys[key] {
			return fmt.Errorf("复核待处理项定位键无效或重复")
		}
		trackedKeys[key] = true
		delete(returnedKeys, key)
		if finding.Status != "open" && finding.Status != "resolved" {
			return fmt.Errorf("复核待处理项状态无效")
		}
		if finding.Status == "resolved" {
			if strings.TrimSpace(finding.CorrectionNote) == "" || finding.ResolvedBy == "" || finding.ResolvedAt == nil || finding.ResolvedAt.IsZero() {
				return fmt.Errorf("已闭环复核意见缺少处置事实")
			}
		} else {
			openKeys[key] = true
		}
	}
	if len(returnedKeys) != 0 {
		return fmt.Errorf("退回复核意见缺少逐项状态")
	}
	for _, finding := range c.OpenFindings {
		key := findingKey(finding.ref())
		if !openKeys[key] {
			return fmt.Errorf("开放复核意见与逐项状态不一致")
		}
		delete(openKeys, key)
	}
	if len(openKeys) != 0 {
		return fmt.Errorf("逐项复核意见未完整反映到开放清单")
	}
	return nil
}
