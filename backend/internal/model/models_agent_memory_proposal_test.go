package model

import (
	"reflect"
	"testing"
	"time"
)

func TestAgentMemorySnapshotPreservesRestoreData(t *testing.T) {
	stamp := time.Date(2026, 10, 1, 2, 3, 4, 0, time.UTC)
	lesson := AgentLesson{
		ID: "memory", AuthorUserID: "owner", Topic: "canvas.write", Category: "canvas",
		Situation: "before writing", Lesson: "read and verify", Source: "user-confirmed",
		OriginRunID: "run", EvidenceJSON: `{"step":"verified"}`, StepsJSON: `[{"tool":"read"}]`,
		Status: AgentLessonStatusApproved, Hits: 3, Injected: 4,
		LastVerifiedAt: &stamp, CreatedAt: stamp, UpdatedAt: stamp,
	}
	restored := SnapshotAgentMemory(lesson).LessonForUser("owner")
	if !reflect.DeepEqual(restored, lesson) {
		t.Fatalf("restoration lost memory data: got %+v", restored)
	}
}

func TestAgentMemoryVersionIgnoresTelemetryAndTimezoneOnly(t *testing.T) {
	stamp := time.Date(2026, 10, 1, 2, 3, 4, 0, time.UTC)
	before := AgentMemorySnapshot{ID: "memory", Source: "source", LastVerifiedAt: &stamp, CreatedAt: stamp, UpdatedAt: stamp}
	after := before
	after.Hits, after.Injected = 10, 20
	local := stamp.In(time.FixedZone("same-instant", 8*60*60))
	after.CreatedAt, after.UpdatedAt, after.LastVerifiedAt = local, local, &local
	if before.ContentVersion() != after.ContentVersion() {
		t.Fatal("usage counters or equivalent timestamps must not invalidate approval")
	}
}

func TestAgentMemoryVersionRejectsChangedContentAndProvenance(t *testing.T) {
	before := AgentMemorySnapshot{ID: "memory", Source: "confirmed", OriginRunID: "run", EvidenceJSON: "{}", Status: AgentLessonStatusApproved}
	for name, change := range map[string]func(*AgentMemorySnapshot){
		"content":   func(s *AgentMemorySnapshot) { s.Lesson = "changed" },
		"source":    func(s *AgentMemorySnapshot) { s.Source = "changed" },
		"origin":    func(s *AgentMemorySnapshot) { s.OriginRunID = "other-run" },
		"evidence":  func(s *AgentMemorySnapshot) { s.EvidenceJSON = `{"changed":true}` },
		"status":    func(s *AgentMemorySnapshot) { s.Status = AgentLessonStatusRejected },
		"recreated": func(s *AgentMemorySnapshot) { s.CreatedAt = time.Now().UTC() },
		"edited":    func(s *AgentMemorySnapshot) { s.UpdatedAt = time.Now().UTC() },
	} {
		t.Run(name, func(t *testing.T) {
			after := before
			change(&after)
			if before.ContentVersion() == after.ContentVersion() {
				t.Fatal("changed memory must invalidate approval and restoration")
			}
		})
	}
}
