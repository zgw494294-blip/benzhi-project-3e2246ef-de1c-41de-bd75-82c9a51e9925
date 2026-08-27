package redaction

import (
	"sort"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

type Engine struct {
	coverageScratch []domain.ConsentCoverageDetail
	segmentScratch  []string
}

func New() *Engine {
	return &Engine{
		coverageScratch: make([]domain.ConsentCoverageDetail, 0, 8),
		segmentScratch:  make([]string, 0, 16),
	}
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
	e.coverageScratch = e.coverageScratch[:0]
	e.coverageScratch = append(e.coverageScratch, c.ConsentCoverage(at).Speakers...)
	return e.coverageScratch
}

func (e *Engine) sortedSegmentIDs(c *domain.CorpusCase) []string {
	e.segmentScratch = e.segmentScratch[:0]
	for id := range c.Segments {
		e.segmentScratch = append(e.segmentScratch, id)
	}
	sort.Slice(e.segmentScratch, func(i, j int) bool {
		left, right := c.Segments[e.segmentScratch[i]], c.Segments[e.segmentScratch[j]]
		if left.RecordingID != right.RecordingID {
			return left.RecordingID < right.RecordingID
		}
		if left.Sequence != right.Sequence {
			return left.Sequence < right.Sequence
		}
		return left.SegmentID < right.SegmentID
	})
	return e.segmentScratch
}
