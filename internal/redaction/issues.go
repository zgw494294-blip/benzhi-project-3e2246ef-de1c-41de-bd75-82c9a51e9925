package redaction

import (
	"fmt"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

type Issue struct {
	Code        string `json:"code"`
	ReviewRound int    `json:"reviewRound,omitempty"`
	SegmentID   string `json:"segmentID,omitempty"`
	MarkID      string `json:"markID,omitempty"`
	ConsentID   string `json:"consentID,omitempty"`
	Message     string `json:"message"`
}

type ReleasePreview struct {
	Available      bool                `json:"available"`
	Status         string              `json:"status"`
	Items          []domain.FrozenItem `json:"items"`
	ItemCount      int                 `json:"itemCount"`
	ManifestDigest string              `json:"manifestDigest,omitempty"`
	Blockers       []Issue             `json:"blockers"`
}

type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "脱敏校验失败"
	}
	return fmt.Sprintf("脱敏校验失败：%s", e.Issues[0].Message)
}
