package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const snapshotVersion = 1

type snapshotEnvelope struct {
	Version    int             `json:"version"`
	Checksum   string          `json:"checksum"`
	Projection json.RawMessage `json:"projection"`
}

func writeSnapshot(path string, state projection) error {
	projectionData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("序列化投影快照: %w", err)
	}
	digest := sha256.Sum256(projectionData)
	envelope := snapshotEnvelope{Version: snapshotVersion, Checksum: hex.EncodeToString(digest[:]), Projection: projectionData}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("封装投影快照: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".projection-*.tmp")
	if err != nil {
		return fmt.Errorf("创建投影临时文件: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if err := writeAll(temporary, data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("替换投影快照: %w", err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func readSnapshot(path string) (projection, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return emptyProjection(), nil
	}
	if err != nil {
		return projection{}, err
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return projection{}, fmt.Errorf("投影快照损坏: %w", err)
	}
	if envelope.Version != snapshotVersion {
		return projection{}, fmt.Errorf("不支持的投影快照版本 %d", envelope.Version)
	}
	digest := sha256.Sum256(envelope.Projection)
	if hex.EncodeToString(digest[:]) != envelope.Checksum {
		return projection{}, fmt.Errorf("投影快照校验和错误")
	}
	state := emptyProjection()
	if err := json.Unmarshal(envelope.Projection, &state); err != nil {
		return projection{}, fmt.Errorf("投影内容损坏: %w", err)
	}
	if state.Cases == nil || state.Idempotency == nil || state.Manifests == nil || state.Credentials == nil {
		return projection{}, fmt.Errorf("投影快照缺少必要集合")
	}
	return state, nil
}
