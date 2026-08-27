package redaction

import (
	"crypto/sha256"
	"encoding/hex"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

func Generate(segment domain.TranscriptSegment, marks []domain.RedactionMark) (domain.RedactedSegment, []Issue) {
	ordered, issues := Normalize(segment, marks)
	if len(issues) > 0 {
		return domain.RedactedSegment{}, issues
	}
	source := []rune(segment.SourceText)
	result := make([]rune, 0, len(source))
	mappings := make([]domain.TextMapping, 0, len(ordered))
	cursor := 0
	for _, mark := range ordered {
		result = append(result, source[cursor:mark.StartRune]...)
		replacement := replacementFor(mark)
		if mark.Action == "retain" {
			replacement = string(source[mark.StartRune:mark.EndRune])
		}
		result = append(result, []rune(replacement)...)
		mappings = append(mappings, domain.TextMapping{
			MarkID: mark.MarkID, SourceStartRune: mark.StartRune,
			SourceEndRune: mark.EndRune, Replacement: replacement,
		})
		cursor = mark.EndRune
	}
	result = append(result, source[cursor:]...)
	digest := sha256.Sum256([]byte(segment.SourceText))
	return domain.RedactedSegment{
		SegmentID: segment.SegmentID, Text: string(result), Mappings: mappings,
		SourceDigest: hex.EncodeToString(digest[:]),
	}, nil
}

func replacementFor(mark domain.RedactionMark) string {
	switch mark.Action {
	case "mask":
		return "[已遮蔽:" + mark.Category + "]"
	case "generalize":
		return mark.ReplacementText
	case "retain":
		return ""
	default:
		return ""
	}
}
