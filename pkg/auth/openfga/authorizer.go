package openfga

import (
	"context"
	"sync"
)

// Authorizer is the minimal interface for authorization checks.
type Authorizer interface {
	Check(ctx context.Context, user, relation, object string) (bool, error)
}

// MemoryAuthorizer is a tiny in-memory authorizer useful for tests and examples.
type MemoryAuthorizer struct {
	mu sync.RWMutex
	// object -> relation -> set(user)
	acl map[string]map[string]map[string]struct{}
}

func NewMemoryAuthorizer() *MemoryAuthorizer {
	return &MemoryAuthorizer{acl: map[string]map[string]map[string]struct{}{}}
}

func (m *MemoryAuthorizer) Allow(user, relation, object string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rels, ok := m.acl[object]
	if !ok {
		rels = map[string]map[string]struct{}{}
		m.acl[object] = rels
	}
	users, ok := rels[relation]
	if !ok {
		users = map[string]struct{}{}
		rels[relation] = users
	}
	users[user] = struct{}{}
}

func (m *MemoryAuthorizer) Deny(user, relation, object string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rels, ok := m.acl[object]; ok {
		if users, ok := rels[relation]; ok {
			delete(users, user)
		}
	}
}

func (m *MemoryAuthorizer) Check(ctx context.Context, user, relation, object string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if rels, ok := m.acl[object]; ok {
		if users, ok := rels[relation]; ok {
			_, ok := users[user]
			return ok, nil
		}
	}
	return false, nil
}
