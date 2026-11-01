package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/catalog"
	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/httpapi"
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

// buildJobs registers handlers and schedules and builds the catalog without
// starting recovery, the worker, or the scheduler. Start the queue only after
// binding the server port, and call Wait before closing the database.
func buildJobs(d *db.DB, logger *slog.Logger, authService *auth.Service, token string) (*jobs.Queue, httpapi.Catalog, httpapi.CatalogSearch) {
	q := jobs.New(d, jobs.Options{Logger: logger})
	var catalogAPI httpapi.Catalog
	var searchAPI httpapi.CatalogSearch
	if token != "" {
		client := tmdb.New(d, tmdb.Options{Token: token, UserAgent: "watcht/" + version, Logger: logger})
		catalogService := catalog.New(d, client, catalog.Options{Jobs: q, Logger: logger})
		q.Register(catalog.JobSyncShow, catalogService.HandleSyncShow)
		q.Register(catalog.JobRefresh, func(ctx context.Context, _ json.RawMessage) error { return catalogService.Refresh(ctx) })
		q.Every(catalog.JobRefresh, time.Hour)
		catalogAPI = catalogService
		searchAPI = client
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
	return q, catalogAPI, searchAPI
}
