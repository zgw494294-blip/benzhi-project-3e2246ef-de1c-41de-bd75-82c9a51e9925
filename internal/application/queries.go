package application

import (
	"context"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
)

func (s *Service) GetCase(ctx context.Context, caseID string) (CaseView, error) {
	if err := ctx.Err(); err != nil {
		return CaseView{}, err
	}
	c, err := s.repository.GetCase(caseID)
	if err != nil {
		return CaseView{}, err
	}
	now := s.now().UTC()
	preview, found := s.cachedReleasePreview(caseID)
	if !found {
		preview, err = s.redactor.ReleasePreview(c, now)
		if err != nil {
			return CaseView{}, err
		}
		s.cacheReleasePreview(caseID, preview)
	}
	return CaseView{
		Case: c, Blockers: s.redactor.Blockers(c, now),
		ConsentCoverage: c.ConsentCoverage(now), ReleasePreview: preview,
	}, nil
}

func (s *Service) cachedReleasePreview(caseID string) (redaction.ReleasePreview, bool) {
	s.previewMu.RLock()
	defer s.previewMu.RUnlock()
	preview, found := s.previews[caseID]
	if !found {
		return redaction.ReleasePreview{}, false
	}
	return cloneReleasePreview(preview), true
}

func (s *Service) cacheReleasePreview(caseID string, preview redaction.ReleasePreview) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.previews[caseID] = cloneReleasePreview(preview)
}

func (s *Service) invalidateReleasePreview(caseID string) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	delete(s.previews, caseID)
}

func cloneReleasePreview(p redaction.ReleasePreview) redaction.ReleasePreview {
	return redaction.ReleasePreview{
		Available:      p.Available,
		Status:         p.Status,
		Items:          append([]domain.FrozenItem(nil), p.Items...),
		ItemCount:      p.ItemCount,
		ManifestDigest: p.ManifestDigest,
		Blockers:       append([]redaction.Issue(nil), p.Blockers...),
	}
}

func (s *Service) VerifyCredential(ctx context.Context, credentialID string) (CredentialVerification, error) {
	if err := ctx.Err(); err != nil {
		return CredentialVerification{}, err
	}
	credential, manifest, err := s.repository.GetCredential(credentialID)
	if err != nil {
		return CredentialVerification{}, err
	}
	valid := redaction.VerifyManifest(manifest) && credential.ManifestDigest == manifest.Digest && credential.FrozenRevision == manifest.FrozenRevision
	reason := "凭据和冻结清单摘要一致"
	if !valid {
		reason = "凭据或冻结清单完整性校验失败"
	}
	return CredentialVerification{Credential: credential, Valid: valid, Reason: reason}, nil
}
