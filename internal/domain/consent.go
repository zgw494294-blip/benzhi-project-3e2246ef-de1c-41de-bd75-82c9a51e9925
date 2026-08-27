package domain

import (
	"sort"
	"strings"
	"time"
)

const ConsentRiskWindow = 30 * 24 * time.Hour

func (c *CorpusCase) AddConsent(consent ConsentRecord, actor string, now time.Time) (Event, error) {
	if err := c.ensureMutable(); err != nil {
		return Event{}, err
	}
	if c.Status != StatusDraft {
		return Event{}, Conflict("intake_already_locked", "受理清单锁定后不可登记授权")
	}
	if strings.TrimSpace(consent.ConsentID) == "" || strings.TrimSpace(consent.SpeakerID) == "" || strings.TrimSpace(consent.EvidenceRef) == "" {
		return Event{}, Validation("invalid_consent", "授权 ID、说话人和证据引用不能为空")
	}
	if consent.ValidFrom.IsZero() || consent.ValidUntil.IsZero() || !consent.ValidUntil.After(consent.ValidFrom) {
		return Event{}, Validation("invalid_consent_period", "授权有效期无效")
	}
	if len(consent.Scope) == 0 {
		return Event{}, Validation("consent_scope_required", "授权范围不能为空")
	}
	if _, exists := c.Consents[consent.ConsentID]; exists {
		return Event{}, Conflict("consent_exists", "授权记录已存在")
	}
	for _, existing := range c.Consents {
		if existing.SpeakerID != consent.SpeakerID || !periodsOverlap(existing.ValidFrom, existing.ValidUntil, consent.ValidFrom, consent.ValidUntil) {
			continue
		}
		if !sameStrings(existing.Scope, consent.Scope) || !sameStrings(existing.Restrictions, consent.Restrictions) {
			return Event{}, Conflict("consent_terms_conflict", "同一说话人的重叠授权期限包含冲突条款")
		}
	}
	consent.CaseID = c.CaseID
	consent.RecordedAt = now.UTC()
	c.Consents[consent.ConsentID] = consent
	return c.record("consent.recorded", actor, now, map[string]any{"consentID": consent.ConsentID, "speakerID": consent.SpeakerID}), nil
}

func (c *CorpusCase) ConsentForSpeaker(speakerID string, at time.Time) (ConsentRecord, bool) {
	detail := c.consentCoverageForSpeaker(speakerID, nil, at.UTC())
	if detail.Status == "valid" {
		return c.Consents[detail.ConsentID], true
	}
	return ConsentRecord{}, false
}

func (c *CorpusCase) ConsentCoverage(at time.Time) ConsentCoverage {
	at = at.UTC()
	segmentsBySpeaker := make(map[string][]string)
	for segmentID, segment := range c.Segments {
		segmentsBySpeaker[segment.SpeakerID] = append(segmentsBySpeaker[segment.SpeakerID], segmentID)
	}
	speakerIDs := make([]string, 0, len(segmentsBySpeaker))
	for speakerID := range segmentsBySpeaker {
		speakerIDs = append(speakerIDs, speakerID)
		sort.Strings(segmentsBySpeaker[speakerID])
	}
	sort.Strings(speakerIDs)
	coverage := ConsentCoverage{AsOf: at, WindowDays: int(ConsentRiskWindow / (24 * time.Hour)), Speakers: make([]ConsentCoverageDetail, 0, len(speakerIDs))}
	coverage.Summary.TotalSpeakers = len(speakerIDs)
	for _, speakerID := range speakerIDs {
		detail := c.consentCoverageForSpeaker(speakerID, segmentsBySpeaker[speakerID], at)
		coverage.Speakers = append(coverage.Speakers, detail)
		if detail.Status == "valid" {
			coverage.Summary.CoveredSpeakers++
		} else {
			coverage.Summary.BlockedSpeakers++
		}
		if detail.ExpiringSoon {
			coverage.Summary.ExpiringSoonSpeakers++
		}
	}
	return coverage
}

func (c *CorpusCase) consentCoverageForSpeaker(speakerID string, segmentIDs []string, at time.Time) ConsentCoverageDetail {
	detail := ConsentCoverageDetail{SpeakerID: speakerID, Status: "missing", BlockerCode: "consent_coverage_missing", AffectedSegmentIDs: append([]string(nil), segmentIDs...)}
	records := make([]ConsentRecord, 0)
	for _, consent := range c.Consents {
		if consent.SpeakerID == speakerID {
			records = append(records, consent)
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ConsentID < records[j].ConsentID })

	eligible := func(consent ConsentRecord) bool {
		return contains(consent.Scope, "research_release") && !hasBlockingRestriction(consent.Restrictions)
	}
	var selected *ConsentRecord
	for index := range records {
		consent := &records[index]
		if !eligible(*consent) || at.Before(consent.ValidFrom) || at.After(consent.ValidUntil) {
			continue
		}
		if selected == nil || consent.ValidUntil.After(selected.ValidUntil) || (consent.ValidUntil.Equal(selected.ValidUntil) && consent.ConsentID < selected.ConsentID) {
			selected = consent
		}
	}
	if selected != nil {
		detail.Status = "valid"
		detail.BlockerCode = ""
		detail.ConsentID = selected.ConsentID
		detail.EvidenceRef = selected.EvidenceRef
		detail.RemainingValiditySeconds = int64(selected.ValidUntil.Sub(at) / time.Second)
		detail.ExpiringSoon = selected.ValidUntil.Sub(at) <= ConsentRiskWindow
		return detail
	}

	selected = nil
	for index := range records {
		consent := &records[index]
		if eligible(*consent) && at.Before(consent.ValidFrom) && (selected == nil || consent.ValidFrom.Before(selected.ValidFrom) || (consent.ValidFrom.Equal(selected.ValidFrom) && consent.ConsentID < selected.ConsentID)) {
			selected = consent
		}
	}
	if selected != nil {
		detail.Status = "not_yet_effective"
		detail.BlockerCode = "consent_not_yet_effective"
		detail.ConsentID = selected.ConsentID
		detail.EvidenceRef = selected.EvidenceRef
		return detail
	}

	selected = nil
	for index := range records {
		consent := &records[index]
		if eligible(*consent) && at.After(consent.ValidUntil) && (selected == nil || consent.ValidUntil.After(selected.ValidUntil) || (consent.ValidUntil.Equal(selected.ValidUntil) && consent.ConsentID < selected.ConsentID)) {
			selected = consent
		}
	}
	if selected != nil {
		detail.Status = "expired"
		detail.BlockerCode = "consent_expired"
		detail.ConsentID = selected.ConsentID
		detail.EvidenceRef = selected.EvidenceRef
		return detail
	}

	if len(records) > 0 {
		selected = &records[0]
		detail.Status = "terms_blocked"
		detail.BlockerCode = "consent_terms_blocked"
		detail.ConsentID = selected.ConsentID
		detail.EvidenceRef = selected.EvidenceRef
	}
	return detail
}

func hasBlockingRestriction(restrictions []string) bool {
	for _, restriction := range restrictions {
		if strings.EqualFold(strings.TrimSpace(restriction), "no_release") {
			return true
		}
	}
	return false
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func periodsOverlap(leftStart, leftEnd, rightStart, rightEnd time.Time) bool {
	return leftStart.Before(rightEnd) && rightStart.Before(leftEnd)
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := map[string]int{}
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
