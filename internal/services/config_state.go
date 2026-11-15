package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/rs/zerolog/log"
)

type ConfigState struct {
	ServerID        int64  `json:"server_id"`
	DesiredRevision int64  `json:"desired_revision"`
	AppliedRevision int64  `json:"applied_revision"`
	Status          string `json:"status"`
	Error           string `json:"error"`
	UpdatedAt       string `json:"updated_at"`
}

type ConfigApplyError struct {
	ServerID int64
	cause    error
}

func (e *ConfigApplyError) Unwrap() error { return e.cause }

func (e *ConfigApplyError) Error() string {
	return fmt.Sprintf("Configuration saved, but server %d has not applied it. Check configuration status and retry synchronization.", e.ServerID)
}

type configAttempt struct {
	ID       string
	Revision int64
	Deadline time.Time
}

func newCommandID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate command identity: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func (s *CaddySyncService) serverLock(id int64) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locks[id] == nil {
		s.locks[id] = &sync.Mutex{}
	}
	return s.locks[id]
}

func (s *CaddySyncService) State(ctx context.Context, id int64) (*ConfigState, error) {
	var state ConfigState
	err := s.db.QueryRowContext(ctx, `SELECT server_id,desired_revision,applied_revision,status,error,updated_at FROM server_config_state WHERE server_id=?`, id).
		Scan(&state.ServerID, &state.DesiredRevision, &state.AppliedRevision, &state.Status, &state.Error, &state.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("read configuration state: %w", err)
	}
	return &state, nil
}

func (s *CaddySyncService) States(ctx context.Context) ([]ConfigState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT server_id,desired_revision,applied_revision,status,error,updated_at FROM server_config_state ORDER BY server_id`)
	if err != nil {
		return nil, fmt.Errorf("read configuration states: %w", err)
	}
	defer rows.Close()
	states := []ConfigState{}
	for rows.Next() {
		var state ConfigState
		if err := rows.Scan(&state.ServerID, &state.DesiredRevision, &state.AppliedRevision, &state.Status, &state.Error, &state.UpdatedAt); err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

func (s *CaddySyncService) mark(ctx context.Context, id, revision int64, status, message string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE server_config_state SET status=?,error=?,updated_at=datetime('now') WHERE server_id=? AND desired_revision=?`, status, message, id, revision)
	if err != nil {
		return fmt.Errorf("record configuration status: %w", err)
	}
	return nil
}

func (s *CaddySyncService) applied(ctx context.Context, id, revision int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE server_config_state SET applied_revision=MAX(applied_revision,?),
	 status=CASE WHEN desired_revision=? THEN 'applied' ELSE 'queued' END,error='',updated_at=datetime('now')
	 WHERE server_id=? AND desired_revision>=?`, revision, revision, id, revision)
	if err != nil {
		return fmt.Errorf("record applied revision: %w", err)
	}
	return nil
}

func (s *CaddySyncService) attemptActive(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.pending[id]
	return ok
}
func (s *CaddySyncService) clearAttempt(id int64, command string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[id].ID == command {
		delete(s.pending, id)
	}
}

func (s *CaddySyncService) commandResult(id int64, result agent.CommandResultPayload) {
	lock := s.serverLock(id)
	lock.Lock()
	defer lock.Unlock()
	s.mu.Lock()
	attempt, ok := s.pending[id]
	s.mu.Unlock()
	if !ok || result.CommandID != attempt.ID || result.Revision != attempt.Revision || result.CommandKind != agent.KindConfigFullSync {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err error
	if result.Success {
		err = s.applied(ctx, id, attempt.Revision)
	} else {
		err = s.mark(ctx, id, attempt.Revision, "failed", "Agent rejected configuration. Check the server and retry synchronization.")
	}
	if err != nil {
		log.Error().Err(err).Int64("server_id", id).Msg("failed to record agent application result")
		return
	}
	s.clearAttempt(id, attempt.ID)
}

// Reconnect replaces connection-local work with reconciliation from durable state.
func (s *CaddySyncService) Reconnect(ctx context.Context, id int64) error {
	lock := s.serverLock(id)
	lock.Lock()
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
	lock.Unlock()
	return s.SyncServer(ctx, id)
}

func (s *CaddySyncService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE server_config_state SET status='queued',error='',updated_at=datetime('now')`); err != nil {
		return fmt.Errorf("queue startup reconciliation: %w", err)
	}
	workerCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			s.reconcile(workerCtx)
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return nil
}

func (s *CaddySyncService) Close() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (s *CaddySyncService) reconcile(ctx context.Context) {
	s.mu.Lock()
	expired := map[int64]configAttempt{}
	for id, attempt := range s.pending {
		if !time.Now().Before(attempt.Deadline) {
			expired[id] = attempt
		}
	}
	s.mu.Unlock()
	for id, attempt := range expired {
		lock := s.serverLock(id)
		lock.Lock()
		s.mu.Lock()
		current, ok := s.pending[id]
		s.mu.Unlock()
		if ok && current.ID == attempt.ID {
			if err := s.mark(ctx, id, attempt.Revision, "failed", "Agent acknowledgment timed out; synchronization will be retried."); err != nil {
				log.Error().Err(err).Msg("record acknowledgment timeout")
			} else {
				s.clearAttempt(id, attempt.ID)
			}
		}
		lock.Unlock()
	}
	states, err := s.States(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Error().Err(err).Msg("read reconciliation work")
		}
		return
	}
	for _, state := range states {
		if ctx.Err() != nil {
			return
		}
		when, timeErr := time.Parse("2006-01-02 15:04:05", state.UpdatedAt)
		if state.Status == "applied" && state.AppliedRevision == state.DesiredRevision && timeErr == nil && time.Since(when) < time.Minute {
			continue
		}
		if state.Status == "failed" {
			when, err := time.Parse("2006-01-02 15:04:05", state.UpdatedAt)
			if err == nil && time.Since(when) < 30*time.Second {
				continue
			}
		}
		if err := s.SyncServer(ctx, state.ServerID); err != nil && ctx.Err() == nil {
			log.Warn().Int64("server_id", state.ServerID).Msg("configuration remains unapplied")
		}
	}
}

func syncServers(ctx context.Context, syncSvc *CaddySyncService, ids ...int64) error {
	if syncSvc == nil {
		return nil
	}
	seen := map[int64]bool{}
	var first error
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			if err := syncSvc.SyncServer(ctx, id); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

func (s *CaddySyncService) SyncDirty(ctx context.Context) error {
	states, err := s.States(ctx)
	if err != nil {
		return err
	}
	var first error
	for _, state := range states {
		if state.Status != "applied" || state.DesiredRevision != state.AppliedRevision {
			if err := s.SyncServer(ctx, state.ServerID); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}
