package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/jobs"
	"github.com/Ezo-mas/watcht/internal/providers"
	"github.com/Ezo-mas/watcht/internal/providers/tmdb"
)

const (
	// jobSessionCleanup deletes sessions past their expiry. Login did this
	// in M2, before the job scheduler existed.
	jobSessionCleanup = "session_cleanup"
	// sessionCleanupInterval is how often expired sessions are deleted.
	sessionCleanupInterval = time.Hour

	// jobProviderCachePrune deletes provider cache entries that expired more
	// than 30 days ago. Newer expired entries stay for stale fallback.
	jobProviderCachePrune = "provider_cache_prune"
	// providerCachePruneInterval is how often old cache entries are deleted.
	providerCachePruneInterval = 24 * time.Hour
)

// startJobs registers the job handlers and schedules, then starts recovery,
// the worker, and the scheduler. They stop when ctx is cancelled; call Wait
// before closing the database. With a token, it also builds and returns the
// catalog service and registers its sync and hourly refresh jobs.
func startJobs(ctx context.Context, d *db.DB, logger *slog.Logger, authService *auth.Service, token string) (*jobs.Queue, *catalog.Service, error) {
	q := jobs.New(d, jobs.Options{Logger: logger})
	var catalogService *catalog.Service
	if token != "" {
		client := tmdb.New(d, tmdb.Options{Token: token, UserAgent: "watcht/" + version, Logger: logger})
		catalogService = catalog.New(d, client, catalog.Options{Jobs: q, Logger: logger})
		q.Register(catalog.JobSyncShow, catalogService.HandleSyncShow)
		q.Register(catalog.JobRefresh, func(ctx context.Context, _ json.RawMessage) error { return catalogService.Refresh(ctx) })
		q.Every(catalog.JobRefresh, time.Hour)
	}
	// Without a token, old catalog jobs fail once as unknown kinds. Refresh
	// catches up when the token is configured again.
	q.Register(jobSessionCleanup, func(ctx context.Context, _ json.RawMessage) error {
		n, err := authService.DeleteExpiredSessions(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("deleted expired sessions", "count", n)
		}
		return nil
	})
	q.Every(jobSessionCleanup, sessionCleanupInterval)
	q.Register(jobProviderCachePrune, func(ctx context.Context, _ json.RawMessage) error {
		n, err := providers.PruneCache(ctx, d, time.Now())
		if err != nil {
			return err
		}
		if n > 0 {
			logger.Info("pruned provider cache", "count", n)
		}
		return nil
	})
	q.Every(jobProviderCachePrune, providerCachePruneInterval)
	if err := q.Start(ctx); err != nil {
		return nil, nil, err
	}
	return q, catalogService, nil
}
