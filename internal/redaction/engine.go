package redaction

import (
	"sort"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Blockers(c *domain.CorpusCase, at time.Time) []Issue {
	issues := make([]Issue, 0)
	coverageBySpeaker := make(map[string]domain.ConsentCoverageDetail)
	for _, detail := range e.coverageDetails(c, at) {
		coverageBySpeaker[detail.SpeakerID] = detail
	}
	segmentIDs := e.sortedSegmentIDs(c)
	for _, segmentID := range segmentIDs {
		segment := c.Segments[segmentID]
		if detail := coverageBySpeaker[segment.SpeakerID]; detail.Status != "valid" {
			issues = append(issues, Issue{Code: detail.BlockerCode, SegmentID: segmentID, ConsentID: detail.ConsentID, Message: "说话人缺少当前可用的研究开放授权"})
		}
		_, markIssues := Normalize(segment, c.Marks[segmentID])
		issues = append(issues, markIssues...)
	}
	return issues
}

func (e *Engine) GenerateAll(c *domain.CorpusCase, at time.Time) (map[string]domain.RedactedSegment, []Issue) {
	issues := e.Blockers(c, at)
	if len(issues) > 0 {
		return nil, issues
	}
	results := make(map[string]domain.RedactedSegment, len(c.Segments))
	for _, segmentID := range e.sortedSegmentIDs(c) {
		result, resultIssues := Generate(c.Segments[segmentID], c.Marks[segmentID])
		if len(resultIssues) > 0 {
			issues = append(issues, resultIssues...)
			continue
		}
		results[segmentID] = result
	}
	if len(issues) > 0 {
		return nil, issues
	}
	return results, nil
}

func (e *Engine) coverageDetails(c *domain.CorpusCase, at time.Time) []domain.ConsentCoverageDetail {
	coverage := c.ConsentCoverage(at).Speakers
	details := make([]domain.ConsentCoverageDetail, len(coverage))
	copy(details, coverage)
	return details
}

func (e *Engine) sortedSegmentIDs(c *domain.CorpusCase) []string {
	segmentIDs := make([]string, 0, len(c.Segments))
	for id := range c.Segments {
		segmentIDs = append(segmentIDs, id)
	}
	sort.Slice(segmentIDs, func(i, j int) bool {
		left, right := c.Segments[segmentIDs[i]], c.Segments[segmentIDs[j]]
		if left.RecordingID != right.RecordingID {
			return left.RecordingID < right.RecordingID
		}
		if left.Sequence != right.Sequence {
			return left.Sequence < right.Sequence
		}
		return left.SegmentID < right.SegmentID
	})
	return segmentIDs
}
