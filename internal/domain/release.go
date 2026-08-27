package domain

import "time"

func (c *CorpusCase) Release(manifest FrozenManifest, credential ReleaseCredential, actor string, now time.Time) (Event, error) {
	if c.Status != StatusReviewApproved {
		return Event{}, Conflict("invalid_status", "仅复核通过的案卷可签发开放凭据")
	}
	if manifest.CaseID != c.CaseID || manifest.Digest == "" || len(manifest.Items) != len(c.Segments) {
		return Event{}, Validation("invalid_manifest", "冻结清单与案卷不一致")
	}
	if credential.CredentialID == "" || credential.ManifestDigest != manifest.Digest || credential.CaseID != c.CaseID {
		return Event{}, Validation("invalid_credential", "开放凭据与冻结清单不一致")
	}
	manifest.FrozenRevision = c.Revision + 1
	manifest.FrozenAt = now.UTC()
	credential.FrozenRevision = manifest.FrozenRevision
	credential.IssuedAt = now.UTC()
	credential.VerificationStatus = "valid"
	c.Manifest = &manifest
	c.Credential = &credential
	c.Status = StatusReleased
	return c.record("release.issued", actor, now, map[string]any{"credentialID": credential.CredentialID, "manifestDigest": manifest.Digest}), nil
}

func (c *CorpusCase) Clone() *CorpusCase {
	clone := *c
	clone.Recordings = cloneMap(c.Recordings)
	clone.Consents = cloneMap(c.Consents)
	clone.Segments = cloneMap(c.Segments)
	clone.Redacted = cloneMap(c.Redacted)
	clone.Marks = make(map[string][]RedactionMark, len(c.Marks))
	for key, marks := range c.Marks {
		clone.Marks[key] = append([]RedactionMark(nil), marks...)
	}
	clone.ReviewHistory = append([]ReviewDecision(nil), c.ReviewHistory...)
	clone.ReviewFindings = append([]ReviewFinding(nil), c.ReviewFindings...)
	clone.OpenFindings = append([]ReviewFinding(nil), c.OpenFindings...)
	clone.Audit = append([]AuditFact(nil), c.Audit...)
	if c.Manifest != nil {
		manifest := *c.Manifest
		manifest.Items = append([]FrozenItem(nil), c.Manifest.Items...)
		clone.Manifest = &manifest
	}
	if c.Credential != nil {
		credential := *c.Credential
		clone.Credential = &credential
	}
	return &clone
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	result := make(map[K]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
