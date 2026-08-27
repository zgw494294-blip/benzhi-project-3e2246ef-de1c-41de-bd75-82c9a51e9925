package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

type immutableRecord struct {
	Kind     string          `json:"kind"`
	ID       string          `json:"id"`
	Payload  json.RawMessage `json:"payload"`
	Checksum string          `json:"checksum"`
}

func appendImmutable(path, kind, id string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	record := immutableRecord{Kind: kind, ID: id, Payload: payload, Checksum: hex.EncodeToString(digest[:])}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("打开只追加记录: %w", err)
	}
	defer file.Close()
	if err := writeAll(file, line); err != nil {
		return err
	}
	return file.Sync()
}
