package domain

import (
	"strings"
	"time"
)

func (c *CorpusCase) AddSegment(segment TranscriptSegment, actor string, now time.Time) (Event, error) {
	if err := c.ensureMutable(); err != nil {
		return Event{}, err
	}
	if c.Status != StatusDraft {
		return Event{}, Conflict("intake_already_locked", "受理清单锁定后不可增删原始片段")
	}
	if segment.SegmentID == "" || segment.RecordingID == "" || segment.SpeakerID == "" || strings.TrimSpace(segment.SourceText) == "" {
		return Event{}, Validation("invalid_segment", "片段 ID、录音 ID、说话人和文本不能为空")
	}
	recording, ok := c.Recordings[segment.RecordingID]
	if !ok {
		return Event{}, Validation("recording_unknown", "片段引用了未知录音")
	}
	if segment.StartMillis < 0 || segment.EndMillis <= segment.StartMillis || segment.EndMillis > recording.DurationMS {
		return Event{}, Validation("invalid_timing", "片段时间定位越界或顺序无效")
	}
	if segment.Sequence <= 0 {
		return Event{}, Validation("invalid_sequence", "片段序号必须为正数")
	}
	for _, existing := range c.Segments {
		if existing.RecordingID == segment.RecordingID && existing.Sequence == segment.Sequence {
			return Event{}, Conflict("sequence_conflict", "同一录音内片段序号重复")
		}
		if existing.RecordingID == segment.RecordingID && segment.StartMillis < existing.EndMillis && existing.StartMillis < segment.EndMillis {
			return Event{}, Conflict("timing_overlap", "同一录音内转写片段时间区间不可重叠")
		}
	}
	if _, exists := c.Segments[segment.SegmentID]; exists {
		return Event{}, Conflict("segment_exists", "片段已存在")
	}
	segment.CaseID = c.CaseID
	c.Segments[segment.SegmentID] = segment
	return c.record("segment.added", actor, now, map[string]any{"segmentID": segment.SegmentID, "sequence": segment.Sequence}), nil
}

func (c *CorpusCase) LockIntake(actor string, now time.Time) (Event, error) {
	if c.Status != StatusDraft {
		return Event{}, Conflict("invalid_status", "仅草稿案卷可锁定受理清单")
	}
	if len(c.Recordings) == 0 || len(c.Segments) == 0 {
		return Event{}, Validation("empty_intake", "至少需要一份录音和一个转写片段")
	}
	coverage := c.ConsentCoverage(now)
	for _, detail := range coverage.Speakers {
		if detail.Status != "valid" {
			business := NewError(KindValidation, detail.BlockerCode, "说话人 "+detail.SpeakerID+" 的研究开放授权当前不可用")
			business.Details = map[string]any{"consentCoverage": detail}
			return Event{}, business
		}
	}
	c.Status = StatusIntakeLocked
	return c.record("intake.locked", actor, now, map[string]any{"segmentCount": len(c.Segments)}), nil
}
