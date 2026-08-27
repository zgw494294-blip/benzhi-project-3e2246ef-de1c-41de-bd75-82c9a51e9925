package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

func reconcileImmutable(directory string, state *projection) error {
	manifestPath := filepath.Join(directory, "manifests.jsonl")
	credentialPath := filepath.Join(directory, "credentials.jsonl")
	manifestRecords, err := scanImmutable(manifestPath, "manifest")
	if err != nil {
		return err
	}
	credentialRecords, err := scanImmutable(credentialPath, "credential")
	if err != nil {
		return err
	}
	for id, payload := range manifestRecords {
		_, exists := state.Manifests[id]
		if !exists {
			return fmt.Errorf("只追加清单 %s 不存在于事件投影", id)
		}
		var recorded domain.FrozenManifest
		if err := json.Unmarshal(payload, &recorded); err != nil {
			return err
		}
		state.Manifests[id] = recorded
	}
	for id, payload := range credentialRecords {
		_, exists := state.Credentials[id]
		if !exists {
			return fmt.Errorf("只追加凭据 %s 不存在于事件投影", id)
		}
		var recorded domain.ReleaseCredential
		if err := json.Unmarshal(payload, &recorded); err != nil {
			return err
		}
		state.Credentials[id] = recorded
	}
	for id, manifest := range state.Manifests {
		if _, exists := manifestRecords[id]; !exists {
			if err := appendImmutable(manifestPath, "manifest", id, manifest); err != nil {
				return err
			}
		}
	}
	for id, credential := range state.Credentials {
		if _, exists := credentialRecords[id]; !exists {
			if err := appendImmutable(credentialPath, "credential", id, credential); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanImmutable(path, expectedKind string) (map[string]json.RawMessage, error) {
	result := map[string]json.RawMessage{}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 64<<10)
	lineNumber := 0
	for {
		line, readErr := reader.ReadBytes('\n')
		if readErr == io.EOF {
			if len(line) == 0 {
				return result, nil
			}
			return result, nil
		}
		if readErr != nil {
			return nil, readErr
		}
		lineNumber++
		if len(line) > maxFrameSize {
			return nil, fmt.Errorf("只追加记录第 %d 行过大", lineNumber)
		}
		var record immutableRecord
		if err := json.Unmarshal(line, &record); err != nil {
			return nil, fmt.Errorf("解析只追加记录第 %d 行: %w", lineNumber, err)
		}
		if record.Kind != expectedKind || record.ID == "" {
			return nil, fmt.Errorf("只追加记录第 %d 行类型或 ID 无效", lineNumber)
		}
		if _, exists := result[record.ID]; exists {
			return nil, fmt.Errorf("只追加记录 %s 重复", record.ID)
		}
		digest := sha256.Sum256(record.Payload)
		if hex.EncodeToString(digest[:]) != record.Checksum {
			return nil, fmt.Errorf("只追加记录 %s 校验和错误", record.ID)
		}
		result[record.ID] = append(json.RawMessage(nil), record.Payload...)
	}
}

func sameJSON(left, right any) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return equalBytes(leftData, rightData)
}
