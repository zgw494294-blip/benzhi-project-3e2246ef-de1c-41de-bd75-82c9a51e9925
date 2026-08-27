package store

import "fmt"

func recoverProjection(log eventLog, snapshotPath string) (projection, error) {
	snapshot, snapshotErr := readSnapshot(snapshotPath)
	if snapshotErr != nil {
		snapshot = emptyProjection()
	}
	replayed := emptyProjection()
	err := log.scan(func(record eventRecord) error {
		if record.Sequence != replayed.LastSequence+1 {
			return fmt.Errorf("事件序号不连续：收到 %d，期望 %d", record.Sequence, replayed.LastSequence+1)
		}
		if record.Case == nil || record.Event.CaseID != record.Case.CaseID || record.Event.Revision != record.Case.Revision {
			return fmt.Errorf("事件与案卷投影不一致，序号 %d", record.Sequence)
		}
		prior := replayed.Cases[record.Case.CaseID]
		if prior != nil && record.Case.Revision != prior.Revision+1 {
			return fmt.Errorf("案卷 %s 修订号不连续", record.Case.CaseID)
		}
		if prior == nil && record.Case.Revision != 1 {
			return fmt.Errorf("案卷首修订号必须为 1")
		}
		if err := record.Case.ValidateProjection(); err != nil {
			return fmt.Errorf("事件序号 %d 的案卷投影无效: %w", record.Sequence, err)
		}
		applyRecord(&replayed, record)
		return nil
	})
	if err != nil {
		return projection{}, err
	}
	if snapshotErr == nil && snapshot.LastSequence == replayed.LastSequence && sameJSON(snapshot, replayed) {
		return snapshot, nil
	}
	return replayed, nil
}

func applyRecord(state *projection, record eventRecord) {
	state.LastSequence = record.Sequence
	state.Cases[record.Case.CaseID] = record.Case.Clone()
	state.Idempotency[idempotencyIndex(record.Cached.CaseID, record.Cached.IdempotencyKey)] = record.Cached
	if record.Manifest != nil {
		state.Manifests[record.Manifest.CaseID] = *record.Manifest
	}
	if record.Credential != nil {
		state.Credentials[record.Credential.CredentialID] = *record.Credential
	}
}
