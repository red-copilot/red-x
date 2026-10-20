package easm

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/store"
)

type GraphWriter interface {
	MergeOrganization(ctx context.Context, org domain.Organization) error
	MergeOrganizationDomain(ctx context.Context, od domain.OrganizationDomain) error
	MergeAsset(ctx context.Context, asset domain.Asset) error
	MergeFinding(ctx context.Context, finding domain.Finding) error
	DeleteEntityEdges(ctx context.Context, entityType, entityID string) error
	Reset(ctx context.Context) error
}

type ProjectionStore interface {
	ClaimGraphOutbox(ctx context.Context, limit int) ([]domain.GraphOutbox, error)
	MarkGraphOutboxProcessed(ctx context.Context, ids []string) error
	GetAssetByID(ctx context.Context, id string) (*domain.Asset, error)
	GetFindingByID(ctx context.Context, id string) (*domain.Finding, error)
	GetOrganizationDomainByID(ctx context.Context, id string) (*domain.OrganizationDomain, error)
	GetOrganization(ctx context.Context, id string) (*domain.Organization, error)
	ListAllOrganizations(ctx context.Context) ([]domain.Organization, error)
	ListAllOrganizationDomains(ctx context.Context, status string) ([]domain.OrganizationDomain, error)
	ListAssets(ctx context.Context, kind, taskID, scopeID string, q store.PageQuery) ([]domain.Asset, int64, error)
	ListFindings(ctx context.Context, kind, taskID, scopeID string, q store.PageQuery) ([]domain.Finding, int64, error)
	ClearGraphOutboxBefore(ctx context.Context, before time.Time) error
}

type Projector struct {
	Store     ProjectionStore
	Writer    GraphWriter
	Log       *slog.Logger
	BatchSize int
}

func NewProjector(st ProjectionStore, w GraphWriter, log *slog.Logger) *Projector {
	return &Projector{Store: st, Writer: w, Log: log, BatchSize: 100}
}

// Run consumes graph outbox events until the context is canceled.
func (p *Projector) Run(ctx context.Context) {
	for {
		n, err := p.ProcessBatch(ctx)
		if err != nil && p.Log != nil {
			p.Log.Warn("graph projection failed", "error", err)
		}
		if ctx.Err() != nil {
			return
		}
		if n > 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (p *Projector) ProcessBatch(ctx context.Context) (int, error) {
	if p.Store == nil || p.Writer == nil {
		return 0, errors.New("graph projector is not configured")
	}
	limit := p.BatchSize
	if limit <= 0 {
		limit = 100
	}
	events, err := p.Store.ClaimGraphOutbox(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := make([]string, 0, len(events))
	var firstErr error
	for _, ev := range events {
		if err := p.process(ctx, ev); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		processed = append(processed, ev.ID)
	}
	if len(processed) > 0 {
		if err := p.Store.MarkGraphOutboxProcessed(ctx, processed); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return len(processed), firstErr
}

func (p *Projector) process(ctx context.Context, ev domain.GraphOutbox) error {
	if err := p.Writer.DeleteEntityEdges(ctx, ev.EntityType, ev.EntityID); err != nil {
		return err
	}
	if ev.Op == domain.OutboxOpDelete {
		return nil
	}
	id, err := domain.ParseID(ev.EntityID)
	if err != nil {
		return err
	}
	switch ev.EntityType {
	case "organization":
		org, err := p.Store.GetOrganization(ctx, id)
		if err != nil || org == nil {
			return err
		}
		return p.Writer.MergeOrganization(ctx, *org)
	case "organization_domain":
		od, err := p.Store.GetOrganizationDomainByID(ctx, id)
		if err != nil || od == nil {
			return err
		}
		return p.Writer.MergeOrganizationDomain(ctx, *od)
	case "asset":
		a, err := p.Store.GetAssetByID(ctx, id)
		if err != nil || a == nil {
			return err
		}
		return p.Writer.MergeAsset(ctx, *a)
	case "finding":
		f, err := p.Store.GetFindingByID(ctx, id)
		if err != nil || f == nil {
			return err
		}
		return p.Writer.MergeFinding(ctx, *f)
	default:
		return nil
	}
}

// Rebuild clears the projection and replays every entity from PostgreSQL.
func (p *Projector) Rebuild(ctx context.Context) error {
	if p.Store == nil || p.Writer == nil {
		return errors.New("graph projector is not configured")
	}
	start := time.Now().UTC()
	if err := p.Writer.Reset(ctx); err != nil {
		return err
	}
	orgs, err := p.Store.ListAllOrganizations(ctx)
	if err != nil {
		return err
	}
	for _, org := range orgs {
		if err := p.Writer.MergeOrganization(ctx, org); err != nil {
			return err
		}
	}
	domains, err := p.Store.ListAllOrganizationDomains(ctx, string(domain.DomainStatusConfirmed))
	if err != nil {
		return err
	}
	for _, od := range domains {
		if err := p.Writer.MergeOrganizationDomain(ctx, od); err != nil {
			return err
		}
	}
	for page := 1; ; page++ {
		items, total, err := p.Store.ListAssets(ctx, "", "", "", store.PageQuery{Page: page, PageSize: 200, Sort: "_id:asc"})
		if err != nil {
			return err
		}
		for _, a := range items {
			if err := p.Writer.MergeAsset(ctx, a); err != nil {
				return err
			}
		}
		if len(items) == 0 || int64(page*200) >= total {
			break
		}
	}
	for page := 1; ; page++ {
		items, total, err := p.Store.ListFindings(ctx, "", "", "", store.PageQuery{Page: page, PageSize: 200, Sort: "_id:asc"})
		if err != nil {
			return err
		}
		for _, f := range items {
			if err := p.Writer.MergeFinding(ctx, f); err != nil {
				return err
			}
		}
		if len(items) == 0 || int64(page*200) >= total {
			break
		}
	}
	return p.Store.ClearGraphOutboxBefore(ctx, start)
}
