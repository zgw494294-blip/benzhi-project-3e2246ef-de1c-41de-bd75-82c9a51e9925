package redaction

import (
	"sort"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/domain"
)

func Normalize(segment domain.TranscriptSegment, marks []domain.RedactionMark) ([]domain.RedactionMark, []Issue) {
	runeCount := len([]rune(segment.SourceText))
	ordered := append([]domain.RedactionMark(nil), marks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StartRune != ordered[j].StartRune {
			return ordered[i].StartRune < ordered[j].StartRune
		}
		if ordered[i].EndRune != ordered[j].EndRune {
			return ordered[i].EndRune < ordered[j].EndRune
		}
		return ordered[i].MarkID < ordered[j].MarkID
	})
	issues := make([]Issue, 0)
	for i, mark := range ordered {
		if mark.StartRune < 0 || mark.EndRune <= mark.StartRune || mark.EndRune > runeCount {
			issues = append(issues, Issue{Code: "mark_out_of_bounds", SegmentID: segment.SegmentID, MarkID: mark.MarkID, Message: "敏感区间超出 Unicode 文本边界"})
		}
		if i > 0 && mark.StartRune < ordered[i-1].EndRune {
			issues = append(issues, Issue{Code: "mark_overlap", SegmentID: segment.SegmentID, MarkID: mark.MarkID, Message: "敏感区间与前一标注重叠"})
		}
		if mark.ResolutionStatus != "resolved" {
			issues = append(issues, Issue{Code: "unresolved_mark", SegmentID: segment.SegmentID, MarkID: mark.MarkID, Message: "敏感标注尚未解决"})
		}
	}
	return ordered, issues
}
