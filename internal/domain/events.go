package domain

import "time"

type Event struct {
	EventID    string         `json:"eventID"`
	CaseID     string         `json:"caseID"`
	Revision   uint64         `json:"revision"`
	Type       string         `json:"type"`
	ActorID    string         `json:"actorID"`
	OccurredAt time.Time      `json:"occurredAt"`
	Facts      map[string]any `json:"facts,omitempty"`
}

type AuditFact struct {
	EventID    string         `json:"eventID"`
	Revision   uint64         `json:"revision"`
	Type       string         `json:"type"`
	ActorID    string         `json:"actorID"`
	OccurredAt time.Time      `json:"occurredAt"`
	Facts      map[string]any `json:"facts,omitempty"`
}

func (c *CorpusCase) record(eventType, actorID string, now time.Time, facts map[string]any) Event {
	c.Revision++
	c.UpdatedAt = now.UTC()
	e := Event{
		EventID: eventID(c.CaseID, c.Revision), CaseID: c.CaseID,
		Revision: c.Revision, Type: eventType, ActorID: actorID,
		OccurredAt: now.UTC(), Facts: facts,
	}
	c.Audit = append(c.Audit, AuditFact{EventID: e.EventID, Revision: e.Revision, Type: e.Type, ActorID: actorID, OccurredAt: e.OccurredAt, Facts: cloneFacts(facts)})
	return e
}

func cloneFacts(facts map[string]any) map[string]any {
	if facts == nil {
		return nil
	}
	cloned := make(map[string]any, len(facts))
	for key, value := range facts {
		cloned[key] = value
	}
	return cloned
}

func eventID(caseID string, revision uint64) string {
	return caseID + "-r" + formatRevision(revision)
}

func formatRevision(value uint64) string {
	if value == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for value > 0 {
		buf = append(buf, byte('0'+value%10))
		value /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
