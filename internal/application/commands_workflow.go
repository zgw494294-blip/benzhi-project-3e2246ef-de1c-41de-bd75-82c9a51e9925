package application

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

func (s *Service) applyCommand(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, *domain.FrozenManifest, *domain.ReleaseCredential, error) {
	var event domain.Event
	var manifest *domain.FrozenManifest
	var credential *domain.ReleaseCredential
	var err error
	switch request.Command {
	case "update_case_metadata":
		event, err = s.updateCaseMetadata(c, request, now)
	case "add_recording":
		event, err = s.addRecording(c, request, now)
	case "add_consent":
		event, err = s.addConsent(c, request, now)
	case "add_segment":
		event, err = s.addSegment(c, request, now)
	case "batch_add_segments":
		event, err = s.batchAddSegments(c, request, now)
	case "lock_intake":
		event, err = c.LockIntake(request.Actor.ID, now)
	case "set_marks":
		event, err = s.setMarks(c, request, now)
	case "generate_redaction":
		event, err = s.generateRedaction(c, request, now)
	case "submit_review":
		event, err = c.SubmitReview(request.Actor.ID, now)
	case "decide_review":
		event, err = s.decideReview(c, request, now)
	case "resolve_review_finding":
		event, err = s.resolveReviewFinding(c, request, now)
	case "release":
		event, manifest, credential, err = s.release(c, request, now)
	default:
		err = domain.Validation("unknown_command", "未知命令 %s", request.Command)
	}
	return event, manifest, credential, err
}

func (s *Service) setMarks(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload SetMarksPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	marks := make([]domain.RedactionMark, 0, len(payload.Marks))
	for _, mark := range payload.Marks {
		marks = append(marks, domain.RedactionMark{
			MarkID: mark.MarkID, Category: mark.Category, StartRune: mark.StartRune, EndRune: mark.EndRune,
			Action: mark.Action, ReplacementText: mark.ReplacementText, Rationale: mark.Rationale, ResolutionStatus: mark.ResolutionStatus,
		})
	}
	refs := make([]domain.ReviewFindingRef, 0, len(payload.FindingRefs))
	for _, ref := range payload.FindingRefs {
		refs = append(refs, domain.ReviewFindingRef{ReviewRound: ref.ReviewRound, SegmentID: ref.SegmentID, MarkID: ref.MarkID, Code: ref.Code})
	}
	return c.SetMarksForFindings(payload.SegmentID, marks, refs, request.Actor.ID, now)
}

func (s *Service) generateRedaction(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload struct{}
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	results, issues := s.redactor.GenerateAll(c, now)
	if len(issues) > 0 {
		business := domain.NewError(domain.KindValidation, "redaction_blocked", "存在未解决的脱敏或授权阻断项")
		business.Details = map[string]any{"issues": issues}
		return domain.Event{}, business
	}
	return c.ApplyRedacted(results, request.Actor.ID, now)
}

func (s *Service) decideReview(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload DecideReviewPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	findings := make([]domain.ReviewFinding, 0, len(payload.Findings))
	for _, finding := range payload.Findings {
		findings = append(findings, domain.ReviewFinding{SegmentID: finding.SegmentID, MarkID: finding.MarkID, Code: finding.Code, Comment: finding.Comment})
	}
	decision := domain.ReviewDecision{
		DecisionID: payload.DecisionID, ReviewerID: payload.ReviewerID,
		SampledSegmentIDs: payload.SampledSegmentIDs, Findings: findings, Outcome: payload.Outcome,
	}
	if payload.ReviewerID != request.Actor.ID {
		return domain.Event{}, domain.Forbidden("reviewer_mismatch", "复核决定中的 reviewerID 必须与操作者一致")
	}
	return c.DecideReview(decision, request.Actor.ID, now)
}

func (s *Service) resolveReviewFinding(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload ResolveReviewFindingPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	return c.ResolveReviewFinding(domain.ReviewFindingRef{
		ReviewRound: payload.ReviewRound, SegmentID: payload.SegmentID, MarkID: payload.MarkID, Code: payload.Code,
	}, payload.CorrectionNote, request.Actor.ID, now)
}

func (s *Service) release(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, *domain.FrozenManifest, *domain.ReleaseCredential, error) {
	var payload ReleasePayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, nil, nil, err
	}
	if payload.IssuedBy == "" || payload.IssuedBy != request.Actor.ID {
		return domain.Event{}, nil, nil, domain.Forbidden("issuer_mismatch", "issuedBy 必须与操作者一致")
	}
	manifest, err := s.redactor.BuildReleaseManifest(c, now)
	if err != nil {
		return domain.Event{}, nil, nil, err
	}
	digest := sha256.Sum256([]byte(c.CaseID + ":" + manifest.Digest + ":" + formatUint(c.Revision+1)))
	credential := domain.ReleaseCredential{
		CredentialID: "rc_" + hex.EncodeToString(digest[:12]), CaseID: c.CaseID,
		ManifestDigest: manifest.Digest, IssuedBy: payload.IssuedBy,
	}
	event, err := c.Release(manifest, credential, request.Actor.ID, now)
	if err != nil {
		return domain.Event{}, nil, nil, err
	}
	return event, c.Manifest, c.Credential, nil
}

func formatUint(value uint64) string {
	if value == 0 {
		return "0"
	}
	buffer := make([]byte, 0, 20)
	for value > 0 {
		buffer = append(buffer, byte('0'+value%10))
		value /= 10
	}
	for i, j := 0, len(buffer)-1; i < j; i, j = i+1, j-1 {
		buffer[i], buffer[j] = buffer[j], buffer[i]
	}
	return string(buffer)
}
