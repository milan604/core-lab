package permissions

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// Metadata contains permission information for authorization.
type Metadata struct {
	ID       string
	Service  string
	BitValue int64
}

// Loader is a function that loads permissions from an external source.
type Loader func(ctx context.Context) (map[string]Metadata, error)

var (
	// ErrLoaderNotConfigured is returned when a loader is not configured.
	ErrLoaderNotConfigured = errors.New("permission loader not configured")
)

// Store manages in-memory permission metadata with thread-safe access.
type Store struct {
	mu     sync.RWMutex
	byCode map[string]Metadata
	loader Loader
}

// NewStore creates a new permission store with an optional loader.
func NewStore(loader Loader) *Store {
	return &Store{
		byCode: make(map[string]Metadata),
		loader: loader,
	}
}

// SetLoader updates the loader function for the store.
func (s *Store) SetLoader(loader Loader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loader = loader
}

// Load fetches permissions using the configured loader and updates the store.
func (s *Store) Load(ctx context.Context) (map[string]Metadata, error) {
	s.mu.RLock()
	loader := s.loader
	s.mu.RUnlock()

	if loader == nil {
		return nil, ErrLoaderNotConfigured
	}

	data, err := loader(ctx)
	if err != nil {
		return nil, err
	}

	s.Replace(data)
	return s.Snapshot(), nil
}

// Replace replaces all permissions in the store with the provided map.
func (s *Store) Replace(perms map[string]Metadata) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(perms) == 0 {
		s.byCode = make(map[string]Metadata)
		return
	}

	updated := make(map[string]Metadata, len(perms))
	for code, meta := range perms {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		updated[trimmed] = meta
	}

	s.byCode = updated
}

// Lookup retrieves permission metadata by code.
func (s *Store) Lookup(code string) (Metadata, bool) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return Metadata{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.byCode[trimmed]
	return meta, ok
}

// ListByService returns all permissions for a given service.
func (s *Store) ListByService(service string) []Metadata {
	normalized := strings.TrimSpace(service)
	if normalized == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Metadata, 0)
	for _, meta := range s.byCode {
		if strings.EqualFold(meta.Service, normalized) {
			result = append(result, meta)
		}
	}

	return result
}

// Snapshot returns a copy of all permissions in the store.
func (s *Store) Snapshot() map[string]Metadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copy := make(map[string]Metadata, len(s.byCode))
	for code, meta := range s.byCode {
		copy[code] = meta
	}

	return copy
}

// Count returns the number of permissions in the store.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byCode)
}
