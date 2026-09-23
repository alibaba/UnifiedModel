package graphstore

import "github.com/alibaba/UnifiedModel/pkg/model"

// WorkspaceSnapshot is raw persisted state, including expired and deleted
// records. Providers use it to share the lifecycle and query implementation
// without replaying writes (which would revive tombstones).
type WorkspaceSnapshot struct {
	UModels   map[string]model.UModelElement
	Entities  map[string]model.EntityPayload
	Relations map[string]model.RelationPayload
}

// NewMemoryStoreFromSnapshot creates an isolated evaluator for persisted state.
// JSON numbers are normalized to the same types used by file.memory.
func NewMemoryStoreFromSnapshot(workspace string, snapshot WorkspaceSnapshot) *MemoryStore {
	s := NewMemoryStore()
	s.umodels[workspace] = normalizeUModelMap(snapshot.UModels)
	s.entities[workspace] = normalizeEntityMap(snapshot.Entities)
	s.relations[workspace] = normalizeRelationMap(snapshot.Relations)
	if s.umodels[workspace] == nil {
		s.umodels[workspace] = make(map[string]model.UModelElement)
	}
	if s.entities[workspace] == nil {
		s.entities[workspace] = make(map[string]model.EntityPayload)
	}
	if s.relations[workspace] == nil {
		s.relations[workspace] = make(map[string]model.RelationPayload)
	}
	return s
}

// SnapshotWorkspace returns raw state without applying query visibility rules.
func (s *MemoryStore) SnapshotWorkspace(workspace string) WorkspaceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return WorkspaceSnapshot{
		UModels:   cloneUModelMap(s.umodels[workspace]),
		Entities:  cloneEntityMap(s.entities[workspace]),
		Relations: cloneRelationMap(s.relations[workspace]),
	}
}
