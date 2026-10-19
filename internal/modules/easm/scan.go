package easm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/red-copilot/red-x/internal/domain"
	"github.com/red-copilot/red-x/internal/modules/tasking"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var ErrDomainNotConfirmed = errors.New("domain is not confirmed for organization")

type ScanSubmitter interface {
	Submit(ctx context.Context, in tasking.SubmitInput) ([]domain.Task, error)
}

type ScanError struct {
	RootDomain string `json:"rootDomain"`
	Error      string `json:"error"`
}

func (s *Service) CreateScans(ctx context.Context, orgID bson.ObjectID, rootDomains []string, options domain.TaskOptions, policyID, scopeID, actor string) ([]domain.Task, []ScanError, error) {
	if s.Store == nil {
		return nil, nil, ErrOrganizationNotFound
	}
	if s.Submitter == nil {
		return nil, nil, errors.New("task submitter is not configured")
	}
	org, err := s.Store.GetOrganization(ctx, orgID)
	if err != nil {
		return nil, nil, err
	}
	if org == nil || org.Status != domain.OrgStatusActive {
		return nil, nil, ErrOrganizationNotFound
	}
	confirmed, err := s.Store.ListOrganizationDomains(ctx, orgID, string(domain.DomainStatusConfirmed))
	if err != nil {
		return nil, nil, err
	}
	allowed := make(map[string]bool, len(confirmed))
	for _, row := range confirmed {
		allowed[row.RootDomain] = true
	}
	var normalized []string
	var invalid []string
	for _, raw := range rootDomains {
		root, err := NormalizeRootDomain(raw)
		if err != nil {
			invalid = append(invalid, strings.TrimSpace(raw))
			continue
		}
		if !allowed[root] {
			invalid = append(invalid, root)
			continue
		}
		normalized = appendUnique(normalized, root)
	}
	if len(normalized) == 0 {
		return nil, nil, fmt.Errorf("%w: no valid confirmed root domains", ErrDomainNotConfirmed)
	}
	if len(invalid) > 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrDomainNotConfirmed, strings.Join(invalid, ", "))
	}
	var tasks []domain.Task
	var scanErrs []ScanError
	for _, root := range normalized {
		created, err := s.Submitter.Submit(ctx, tasking.SubmitInput{
			Name: "EASM " + org.CanonicalName + " " + root, Kind: domain.TaskDomain,
			Target: root, Options: options, PolicyID: policyID, ScopeID: scopeID,
			OrganizationID: orgID.Hex(), RootDomain: root, Origin: domain.TaskOriginEASM,
			CreatedBy: actor,
		})
		if err != nil {
			scanErrs = append(scanErrs, ScanError{RootDomain: root, Error: err.Error()})
			continue
		}
		tasks = append(tasks, created...)
	}
	if len(tasks) > 0 {
		s.audit(ctx, actor, "easm.scan_create", orgID.Hex(), map[string]any{
			"domains": normalized, "tasks": len(tasks), "errors": len(scanErrs),
		})
	}
	if len(tasks) == 0 && len(scanErrs) > 0 {
		return nil, scanErrs, errors.New("no scan tasks were created")
	}
	return tasks, scanErrs, nil
}
