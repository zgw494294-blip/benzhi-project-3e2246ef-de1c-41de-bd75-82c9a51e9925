package store

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const maxFrameSize = 32 << 20

type eventLog struct{ path string }

func (l eventLog) append(record eventRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化事件记录: %w", err)
	}
	if len(payload) > maxFrameSize {
		return fmt.Errorf("事件帧超过限制")
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(payload)))
	checksum := sha256.Sum256(payload)
	for _, part := range [][]byte{header, payload, checksum[:]} {
		if err := writeAll(file, part); err != nil {
			return fmt.Errorf("追加事件日志: %w", err)
		}
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("同步事件日志: %w", err)
	}
	return nil
}

func (l eventLog) scan(apply func(eventRecord) error) error {
	file, err := os.OpenFile(l.path, os.O_RDWR, 0600)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	var offset int64
	for {
		frameStart := offset
		header := make([]byte, 4)
		n, readErr := io.ReadFull(file, header)
		if readErr == io.EOF {
			return nil
		}
		if readErr == io.ErrUnexpectedEOF {
			return truncateTail(file, frameStart)
		}
		if readErr != nil {
			return fmt.Errorf("读取事件头 offset=%d: %w", offset, readErr)
		}
		offset += int64(n)
		length := binary.BigEndian.Uint32(header)
		if length == 0 || length > maxFrameSize {
			return fmt.Errorf("无效事件帧长度 offset=%d", offset-4)
		}
		frame := make([]byte, int(length)+sha256.Size)
		n, readErr = io.ReadFull(file, frame)
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			return truncateTail(file, frameStart)
		}
		if readErr != nil {
			return fmt.Errorf("读取事件帧 offset=%d: %w", offset, readErr)
		}
		offset += int64(n)
		payload, recordedChecksum := frame[:length], frame[length:]
		actual := sha256.Sum256(payload)
		if !equalBytes(actual[:], recordedChecksum) {
			return fmt.Errorf("事件帧校验和错误 offset=%d", offset-int64(len(frame))-4)
		}
		var record eventRecord
		if err := json.Unmarshal(payload, &record); err != nil {
			return fmt.Errorf("解析事件帧 offset=%d: %w", offset, err)
		}
		if err := apply(record); err != nil {
			return err
		}
	}
}

func truncateTail(file *os.File, validLength int64) error {
	if err := file.Truncate(validLength); err != nil {
		return fmt.Errorf("截断未完成事件尾帧: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("同步事件日志截断: %w", err)
	}
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for i := range left {
		different |= left[i] ^ right[i]
	}
	return different == 0
}
