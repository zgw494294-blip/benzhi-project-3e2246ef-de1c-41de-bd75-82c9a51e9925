package application

import (
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

const MaxBatchSegments = domain.MaxSegmentBatchSize

func (s *Service) createCase(request CommandRequest, now time.Time) (*domain.CorpusCase, domain.Event, error) {
	var payload CreateCasePayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, domain.Event{}, err
	}
	return domain.NewCase(request.CaseID, payload.Title, payload.LanguageCode, payload.CollectionContext, payload.OwnerID, request.Actor.ID, now)
}

func (s *Service) addRecording(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload AddRecordingPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	return c.AddRecording(domain.Recording{RecordingID: payload.RecordingID, Label: payload.Label, DurationMS: payload.DurationMS, EvidenceRef: payload.EvidenceRef}, request.Actor.ID, now)
}

func (s *Service) updateCaseMetadata(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload UpdateCaseMetadataPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	return c.UpdateMetadata(domain.MetadataPatch{
		Title: payload.Title.Pointer(), LanguageCode: payload.LanguageCode.Pointer(),
		CollectionContext: payload.CollectionContext.Pointer(), OwnerID: payload.OwnerID.Pointer(),
	}, request.Actor.ID, now)
}

func (s *Service) addConsent(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload AddConsentPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	return c.AddConsent(domain.ConsentRecord{
		ConsentID: payload.ConsentID, SpeakerID: payload.SpeakerID, Scope: payload.Scope,
		Restrictions: payload.Restrictions, ValidFrom: payload.ValidFrom, ValidUntil: payload.ValidUntil,
		EvidenceRef: payload.EvidenceRef,
	}, request.Actor.ID, now)
}

func (s *Service) addSegment(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload AddSegmentPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	return c.AddSegment(domain.TranscriptSegment{
		SegmentID: payload.SegmentID, RecordingID: payload.RecordingID, SpeakerID: payload.SpeakerID,
		StartMillis: payload.StartMillis, EndMillis: payload.EndMillis, SourceText: payload.SourceText, Sequence: payload.Sequence,
	}, request.Actor.ID, now)
}

func (s *Service) batchAddSegments(c *domain.CorpusCase, request CommandRequest, now time.Time) (domain.Event, error) {
	var payload BatchAddSegmentsPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return domain.Event{}, err
	}
	if len(payload.Segments) > MaxBatchSegments {
		return domain.Event{}, domain.Validation("batch_segments_limit_exceeded", "每批最多登记 %d 个片段", MaxBatchSegments)
	}
	segments := make([]domain.TranscriptSegment, 0, len(payload.Segments))
	for _, item := range payload.Segments {
		segments = append(segments, domain.TranscriptSegment{
			SegmentID: item.SegmentID, RecordingID: item.RecordingID, SpeakerID: item.SpeakerID,
			StartMillis: item.StartMillis, EndMillis: item.EndMillis, SourceText: item.SourceText, Sequence: item.Sequence,
		})
	}
	return c.AddSegments(segments, request.Actor.ID, now)
}
