package application

import (
	"context"

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
	preview, err := s.redactor.ReleasePreview(c, now)
	if err != nil {
		return CaseView{}, err
	}
	return CaseView{
		Case: c, Blockers: s.redactor.Blockers(c, now),
		ConsentCoverage: c.ConsentCoverage(now), ReleasePreview: preview,
	}, nil
}

func (s *Service) VerifyCredential(ctx context.Context, credentialID string) (CredentialVerification, error) {
	if err := ctx.Err(); err != nil {
		return CredentialVerification{}, err
	}
	credential, manifest, err := s.repository.GetCredential(credentialID)
	if err != nil {
		return CredentialVerification{}, err
	}
	// 凭据有效性必须依据一致且未被篡改的案卷事实判定：凭据与冻结清单需归属同一案卷，
	// 清单摘要可由冻结项重新计算，且凭据摘要、冻结修订号均与冻结清单一致。
	valid := credential.CaseID == manifest.CaseID &&
		redaction.VerifyManifest(manifest) &&
		credential.ManifestDigest == manifest.Digest &&
		credential.FrozenRevision == manifest.FrozenRevision
	reason := "凭据和冻结清单摘要一致"
	if !valid {
		reason = "凭据或冻结清单完整性校验失败"
	}
	return CredentialVerification{Credential: credential, Valid: valid, Reason: reason}, nil
}
