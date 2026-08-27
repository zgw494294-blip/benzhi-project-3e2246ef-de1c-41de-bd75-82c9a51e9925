package domain

import (
	"sort"
	"strings"
	"time"
)

const MaxSegmentBatchSize = 100

type BatchSegmentIssue struct {
	Index     int    `json:"index"`
	SegmentID string `json:"segmentID,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

func (c *CorpusCase) AddSegments(segments []TranscriptSegment, actor string, now time.Time) (Event, error) {
	if err := c.ensureMutable(); err != nil {
		return Event{}, err
	}
	if c.Status != StatusDraft {
		return Event{}, Conflict("intake_already_locked", "受理清单锁定后不可增删原始片段")
	}
	if len(segments) == 0 {
		return Event{}, Validation("batch_segments_required", "批量登记至少需要一个片段")
	}
	if len(segments) > MaxSegmentBatchSize {
		return Event{}, Validation("batch_segments_limit_exceeded", "每批最多登记 %d 个片段", MaxSegmentBatchSize)
	}

	issues := validateSegmentBatch(c, segments)
	if len(issues) > 0 {
		business := NewError(KindValidation, "batch_segments_invalid", "批量片段登记校验失败")
		business.Details = map[string]any{"issues": issues}
		return Event{}, business
	}

	segmentIDs := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment.CaseID = c.CaseID
		c.Segments[segment.SegmentID] = segment
		segmentIDs = append(segmentIDs, segment.SegmentID)
	}
	sort.Strings(segmentIDs)
	return c.record("segment.batch_added", actor, now, map[string]any{"segmentCount": len(segments), "segmentIDs": segmentIDs}), nil
}

func validateSegmentBatch(c *CorpusCase, segments []TranscriptSegment) []BatchSegmentIssue {
	issues := make([]BatchSegmentIssue, 0)
	issueCodes := map[int]map[string]bool{}
	seenIDs := map[string]int{}
	type position struct {
		recordingID string
		sequence    int
	}
	seenPositions := map[position]int{}

	add := func(index int, segmentID, code, message string) {
		if issueCodes[index] == nil {
			issueCodes[index] = map[string]bool{}
		}
		if issueCodes[index][code] {
			return
		}
		issueCodes[index][code] = true
		issues = append(issues, BatchSegmentIssue{Index: index, SegmentID: segmentID, Code: code, Message: message})
	}

	for index, segment := range segments {
		if strings.TrimSpace(segment.SegmentID) == "" {
			add(index, segment.SegmentID, "segment_id_required", "片段 ID 不能为空")
		} else {
			if _, exists := c.Segments[segment.SegmentID]; exists {
				add(index, segment.SegmentID, "segment_exists", "片段 ID 已存在")
			}
			if prior, exists := seenIDs[segment.SegmentID]; exists {
				add(index, segment.SegmentID, "batch_segment_id_duplicate", "片段 ID 与批次索引 "+formatIndex(prior)+" 重复")
			} else {
				seenIDs[segment.SegmentID] = index
			}
		}
		if strings.TrimSpace(segment.RecordingID) == "" {
			add(index, segment.SegmentID, "recording_id_required", "录音 ID 不能为空")
		}
		if strings.TrimSpace(segment.SpeakerID) == "" {
			add(index, segment.SegmentID, "speaker_id_required", "说话人标识不能为空")
		}
		if strings.TrimSpace(segment.SourceText) == "" {
			add(index, segment.SegmentID, "source_text_required", "转写文本不能为空")
		}
		if segment.Sequence <= 0 {
			add(index, segment.SegmentID, "invalid_sequence", "片段序号必须为正数")
		}

		recording, recordingExists := c.Recordings[segment.RecordingID]
		if segment.RecordingID != "" && !recordingExists {
			add(index, segment.SegmentID, "recording_unknown", "片段引用了未知录音")
		}
		if recordingExists && (segment.StartMillis < 0 || segment.EndMillis <= segment.StartMillis || segment.EndMillis > recording.DurationMS) {
			add(index, segment.SegmentID, "invalid_timing", "片段时间定位越界或顺序无效")
		}

		if segment.RecordingID != "" && segment.Sequence > 0 {
			key := position{recordingID: segment.RecordingID, sequence: segment.Sequence}
			if prior, exists := seenPositions[key]; exists {
				add(index, segment.SegmentID, "batch_sequence_conflict", "同一录音内序号与批次索引 "+formatIndex(prior)+" 重复")
			} else {
				seenPositions[key] = index
			}
			for _, existing := range c.Segments {
				if existing.RecordingID == segment.RecordingID && existing.Sequence == segment.Sequence {
					add(index, segment.SegmentID, "sequence_conflict", "同一录音内片段序号与既有片段重复")
					break
				}
			}
		}

		if segment.RecordingID != "" && segment.StartMillis >= 0 && segment.EndMillis > segment.StartMillis {
			for priorIndex := 0; priorIndex < index; priorIndex++ {
				prior := segments[priorIndex]
				if prior.RecordingID == segment.RecordingID && intervalsOverlap(segment.StartMillis, segment.EndMillis, prior.StartMillis, prior.EndMillis) {
					add(index, segment.SegmentID, "batch_timing_overlap", "时间区间与批次索引 "+formatIndex(priorIndex)+" 重叠")
				}
			}
			for _, existing := range c.Segments {
				if existing.RecordingID == segment.RecordingID && intervalsOverlap(segment.StartMillis, segment.EndMillis, existing.StartMillis, existing.EndMillis) {
					add(index, segment.SegmentID, "timing_overlap", "时间区间与既有片段重叠")
				}
			}
		}
	}
	return issues
}

func intervalsOverlap(leftStart, leftEnd, rightStart, rightEnd int64) bool {
	return leftStart < rightEnd && rightStart < leftEnd
}

func formatIndex(value int) string {
	if value == 0 {
		return "0"
	}
	buffer := make([]byte, 0, 10)
	for value > 0 {
		buffer = append(buffer, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(buffer)-1; left < right; left, right = left+1, right-1 {
		buffer[left], buffer[right] = buffer[right], buffer[left]
	}
	return string(buffer)
}
