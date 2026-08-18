package runtimeplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"leaflag/backend/internal/model"
)

func TestStore_EvaluatesAndReadsOnlyFromSnapshot(t *testing.T) {
	projectID, environmentID := uuid.New(), uuid.New()
	rawKey := "leaflag_sk_runtime"
	sum := sha256.Sum256([]byte(rawKey))
	store := NewStore()
	require.NoError(t, store.Replace(&Snapshot{
		APIKeys: []APIKey{{KeyHash: hex.EncodeToString(sum[:]), ProjectID: projectID, EnvironmentID: environmentID}},
		Flags: []model.FeatureFlag{{
			ProjectID: projectID, Key: "checkout",
			Configs: []model.FlagEnvironmentConfig{{EnvironmentID: environmentID, Enabled: true}},
			Strategies: []model.FlagStrategy{{
				EnvironmentID: environmentID, IsDefault: true, DefaultVariant: "on",
				Variants: []model.FlagStrategyVariant{{Key: "on", Value: datatypes.JSON([]byte("true"))}, {Key: "off", Value: datatypes.JSON([]byte("false"))}},
			}},
		}},
		Parameters: []RuntimeParameter{{ProjectID: projectID, EnvironmentID: environmentID, Key: "service/db/host", Value: "db.internal", Version: 3}},
	}, `"snapshot-1"`))

	resolvedProject, resolvedEnvironment, err := store.ResolveEnvironment(context.Background(), rawKey)
	require.NoError(t, err)
	assert.Equal(t, projectID, resolvedProject)
	assert.Equal(t, environmentID, resolvedEnvironment)
	assert.Equal(t, true, store.Evaluate(context.Background(), projectID, environmentID, "checkout", nil).Value)
	param, ok := store.GetParameter(projectID, environmentID, "service/db/host")
	require.True(t, ok)
	assert.Equal(t, "db.internal", param.Value)

	require.NoError(t, store.Replace(&Snapshot{}, `"snapshot-2"`))
	_, _, err = store.ResolveEnvironment(context.Background(), rawKey)
	assert.Error(t, err)
}

type scriptedSource struct {
	loads []sourceResult
}

type sourceResult struct {
	snapshot    *Snapshot
	etag        string
	notModified bool
	err         error
}

func (s *scriptedSource) Load(_ context.Context, _ string) (*Snapshot, string, bool, error) {
	if len(s.loads) == 0 {
		return nil, "", false, errors.New("unexpected load")
	}
	result := s.loads[0]
	s.loads = s.loads[1:]
	return result.snapshot, result.etag, result.notModified, result.err
}

func TestSyncer_PreservesLastSnapshotAfterFailure(t *testing.T) {
	store := NewStore()
	source := &scriptedSource{loads: []sourceResult{
		{snapshot: &Snapshot{}, etag: `"one"`},
		{err: errors.New("control plane unavailable")},
	}}
	syncer := Syncer{Store: store, Source: source}
	require.NoError(t, syncer.SyncOnce(context.Background()))
	require.Error(t, syncer.SyncOnce(context.Background()))
	status := store.Status()
	assert.True(t, status.Ready)
	assert.Contains(t, status.LastError, "control plane unavailable")
	version, err := store.Version(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, `"one"`, version)
}
