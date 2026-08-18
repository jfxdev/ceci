// Package runtimeplane contains the database-free state and synchronisation
// machinery used by leaflag's Data Plane.
package runtimeplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/service/flag"
)

// Snapshot is the complete read model replicated from the Control Plane. API
// keys contain only their sha256 hash, never the raw token.
type Snapshot struct {
	APIKeys    []APIKey            `json:"apiKeys"`
	Flags      []model.FeatureFlag `json:"flags"`
	Parameters []RuntimeParameter  `json:"parameters"`
}

type APIKey struct {
	KeyHash       string    `json:"keyHash"`
	ProjectID     uuid.UUID `json:"projectId"`
	EnvironmentID uuid.UUID `json:"environmentId"`
}

type RuntimeParameter struct {
	ProjectID     uuid.UUID `json:"projectId"`
	EnvironmentID uuid.UUID `json:"environmentId"`
	Key           string    `json:"key"`
	Value         string    `json:"value"`
	Version       int       `json:"version"`
}

// Source serves snapshots to a synchronizer. The control-plane HTTP client
// and the all-in-one in-process adapter both implement this interface.
type Source interface {
	Load(ctx context.Context, ifNoneMatch string) (snapshot *Snapshot, etag string, notModified bool, err error)
}

type scope struct {
	projectID     uuid.UUID
	environmentID uuid.UUID
}

type loadedScope struct {
	flags      []model.FeatureFlag
	parameters map[string]RuntimeParameter
}

type state struct {
	etag   string
	scopes map[scope]loadedScope
	keys   map[string]scope
}

// Store owns a compiled snapshot. It swaps state atomically so runtime
// requests never observe a partially decoded or partially compiled update.
type Store struct {
	current atomic.Pointer[state]

	mu          sync.Mutex
	lastSuccess time.Time
	lastError   string
	subscribers map[uuid.UUID]map[chan struct{}]struct{}
}

func NewStore() *Store {
	return &Store{subscribers: make(map[uuid.UUID]map[chan struct{}]struct{})}
}

func (s *Store) Replace(snapshot *Snapshot, etag string) error {
	if snapshot == nil {
		return errors.New("runtime snapshot is nil")
	}
	compiled, err := compileSnapshot(snapshot, etag)
	if err != nil {
		return err
	}
	previous := s.current.Swap(compiled)

	s.mu.Lock()
	s.lastSuccess = time.Now().UTC()
	s.lastError = ""
	for projectID, listeners := range s.subscribers {
		if previous == nil || projectChanged(previous, compiled, projectID) {
			for ch := range listeners {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}
	s.mu.Unlock()
	return nil
}

func compileSnapshot(snapshot *Snapshot, etag string) (*state, error) {
	compiled := &state{etag: etag, keys: make(map[string]scope), scopes: make(map[scope]loadedScope)}
	for _, key := range snapshot.APIKeys {
		if key.KeyHash == "" || key.ProjectID == uuid.Nil || key.EnvironmentID == uuid.Nil {
			return nil, errors.New("runtime snapshot contains an invalid API key")
		}
		sc := scope{projectID: key.ProjectID, environmentID: key.EnvironmentID}
		if existing, exists := compiled.keys[key.KeyHash]; exists && existing != sc {
			return nil, errors.New("runtime snapshot maps one API key hash to multiple scopes")
		}
		compiled.keys[key.KeyHash] = sc
		if _, exists := compiled.scopes[sc]; !exists {
			compiled.scopes[sc] = loadedScope{parameters: make(map[string]RuntimeParameter)}
		}
	}

	for sc, loaded := range compiled.scopes {
		for _, flag := range snapshot.Flags {
			if flag.ProjectID != sc.projectID {
				continue
			}
			loaded.flags = append(loaded.flags, filterFlagForEnvironment(flag, sc.environmentID))
		}
		for _, param := range snapshot.Parameters {
			if param.ProjectID == sc.projectID && param.EnvironmentID == sc.environmentID {
				loaded.parameters[param.Key] = param
			}
		}
		compiled.scopes[sc] = loaded
	}
	return compiled, nil
}

func filterFlagForEnvironment(flag model.FeatureFlag, environmentID uuid.UUID) model.FeatureFlag {
	filtered := flag
	filtered.Configs = nil
	filtered.Strategies = nil
	for _, config := range flag.Configs {
		if config.EnvironmentID == environmentID {
			filtered.Configs = append(filtered.Configs, config)
		}
	}
	for _, strategy := range flag.Strategies {
		if strategy.EnvironmentID == environmentID {
			filtered.Strategies = append(filtered.Strategies, strategy)
		}
	}
	return filtered
}

func projectChanged(previous, next *state, projectID uuid.UUID) bool {
	if previous.etag == next.etag {
		return false
	}
	return true
}

// ResolveEnvironment validates a raw runtime API key from its locally stored
// hash, satisfying middleware.ProjectKeyResolver without a database lookup.
func (s *Store) ResolveEnvironment(_ context.Context, rawKey string) (uuid.UUID, uuid.UUID, error) {
	current := s.current.Load()
	if current == nil {
		return uuid.Nil, uuid.Nil, errors.New("data plane has no runtime snapshot")
	}
	sum := sha256.Sum256([]byte(rawKey))
	sc, ok := current.keys[hex.EncodeToString(sum[:])]
	if !ok {
		return uuid.Nil, uuid.Nil, errors.New("runtime API key not found")
	}
	return sc.projectID, sc.environmentID, nil
}

func (s *Store) Evaluate(_ context.Context, projectID, environmentID uuid.UUID, key string, evalCtx map[string]any) flag.EvaluationResult {
	loaded, ok := s.scope(projectID, environmentID)
	if !ok {
		return flag.EvaluationResult{Key: key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeFlagNotFound}
	}
	return flag.EvaluateLoaded(loaded.flags, key, evalCtx)
}

func (s *Store) EvaluateAll(_ context.Context, projectID, environmentID uuid.UUID, evalCtx map[string]any) ([]flag.EvaluationResult, error) {
	loaded, ok := s.scope(projectID, environmentID)
	if !ok {
		return nil, fmt.Errorf("runtime scope not found")
	}
	return flag.EvaluateAllLoaded(loaded.flags, evalCtx), nil
}

func (s *Store) Version(_ context.Context, _ uuid.UUID) (string, error) {
	current := s.current.Load()
	if current == nil {
		return "", errors.New("data plane has no runtime snapshot")
	}
	return current.etag, nil
}

func (s *Store) GetParameter(projectID, environmentID uuid.UUID, key string) (RuntimeParameter, bool) {
	loaded, ok := s.scope(projectID, environmentID)
	if !ok {
		return RuntimeParameter{}, false
	}
	param, ok := loaded.parameters[key]
	return param, ok
}

func (s *Store) ListParameters(projectID, environmentID uuid.UUID, prefix string) []RuntimeParameter {
	loaded, ok := s.scope(projectID, environmentID)
	if !ok {
		return nil
	}
	out := make([]RuntimeParameter, 0)
	for key, param := range loaded.parameters {
		if len(prefix) == 0 || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, param)
		}
	}
	// Stable ordering is needed both for Consul-like responses and tests.
	sortParameters(out)
	return out
}

func sortParameters(params []RuntimeParameter) {
	for i := 1; i < len(params); i++ {
		for j := i; j > 0 && params[j].Key < params[j-1].Key; j-- {
			params[j], params[j-1] = params[j-1], params[j]
		}
	}
}

func (s *Store) scope(projectID, environmentID uuid.UUID) (loadedScope, bool) {
	current := s.current.Load()
	if current == nil {
		return loadedScope{}, false
	}
	loaded, ok := current.scopes[scope{projectID: projectID, environmentID: environmentID}]
	return loaded, ok
}

func (s *Store) Subscribe(projectID uuid.UUID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	if s.subscribers[projectID] == nil {
		s.subscribers[projectID] = make(map[chan struct{}]struct{})
	}
	s.subscribers[projectID][ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subscribers[projectID], ch)
		if len(s.subscribers[projectID]) == 0 {
			delete(s.subscribers, projectID)
		}
		s.mu.Unlock()
	}
}

type Status struct {
	Ready       bool
	LastSuccess time.Time
	LastError   string
}

func (s *Store) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Status{Ready: s.current.Load() != nil, LastSuccess: s.lastSuccess, LastError: s.lastError}
}

func (s *Store) SetSyncError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.lastError = ""
		return
	}
	s.lastError = err.Error()
}
