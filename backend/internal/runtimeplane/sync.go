package runtimeplane

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPSource struct {
	URL    string
	Token  string
	Client *http.Client
}

func (s HTTPSource) Load(ctx context.Context, ifNoneMatch string) (*Snapshot, string, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.URL, "/")+"/internal/v1/runtime/snapshot", nil)
	if err != nil {
		return nil, "", false, err
	}
	request.Header.Set("Authorization", "Bearer "+s.Token)
	if ifNoneMatch != "" {
		request.Header.Set("If-None-Match", ifNoneMatch)
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, "", false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		return nil, ifNoneMatch, true, nil
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return nil, "", false, fmt.Errorf("runtime snapshot request failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	etag := response.Header.Get("ETag")
	if etag == "" {
		return nil, "", false, fmt.Errorf("runtime snapshot response is missing ETag")
	}
	var snapshot Snapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		return nil, "", false, fmt.Errorf("decode runtime snapshot: %w", err)
	}
	return &snapshot, etag, false, nil
}

type Syncer struct {
	Store    *Store
	Source   Source
	Interval time.Duration
}

func (s Syncer) Run(ctx context.Context) {
	s.SyncOnce(ctx)
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SyncOnce(ctx)
		}
	}
}

func (s Syncer) SyncOnce(ctx context.Context) error {
	if s.Store == nil || s.Source == nil {
		return fmt.Errorf("runtime syncer requires a store and source")
	}
	etag := ""
	if current := s.Store.current.Load(); current != nil {
		etag = current.etag
	}
	snapshot, newETag, notModified, err := s.Source.Load(ctx, etag)
	if err != nil {
		s.Store.SetSyncError(err)
		return err
	}
	if notModified {
		s.Store.mu.Lock()
		s.Store.lastSuccess = time.Now().UTC()
		s.Store.lastError = ""
		s.Store.mu.Unlock()
		return nil
	}
	if err := s.Store.Replace(snapshot, newETag); err != nil {
		s.Store.SetSyncError(err)
		return err
	}
	return nil
}
