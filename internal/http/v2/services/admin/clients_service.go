// Package admin provee servicios para operaciones administrativas HTTP V2.
package admin

import (
	"context"
	"fmt"

	"github.com/dropDatabas3/hellojohn/internal/controlplane/v2"
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// ClientService define las operaciones de clients para el admin API.
type ClientService interface {
	List(ctx context.Context, tenantSlug string) ([]repository.Client, error)
	Create(ctx context.Context, tenantSlug string, input controlplane.ClientInput) (*repository.Client, error)
	Update(ctx context.Context, tenantSlug string, input controlplane.ClientInput) (*repository.Client, error)
	Delete(ctx context.Context, tenantSlug, clientID string) error
}

// clientService implementa ClientService usando controlplane.Service.
type clientService struct {
	cp controlplane.Service
}

// NewClientService crea un nuevo servicio de clients.
func NewClientService(cp controlplane.Service) ClientService {
	return &clientService{cp: cp}
}

func (s *clientService) List(ctx context.Context, tenantSlug string) ([]repository.Client, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("admin.clients"),
		logger.Op("List"),
		logger.TenantSlug(tenantSlug),
	)

	clients, err := s.cp.ListClients(ctx, tenantSlug)
	if err != nil {
		log.Error("failed to list clients", logger.Err(err))
		return nil, err
	}

	log.Debug("clients listed", logger.Int("count", len(clients)))
	return clients, nil
}

func (s *clientService) Create(ctx context.Context, tenantSlug string, input controlplane.ClientInput) (*repository.Client, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("admin.clients"),
		logger.Op("Create"),
		logger.TenantSlug(tenantSlug),
		logger.ClientID(input.ClientID),
	)

	if input.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}

	client, err := s.cp.CreateClient(ctx, tenantSlug, input)
	if err != nil {
		log.Error("failed to create client", logger.Err(err))
		return nil, err
	}

	log.Info("client created", logger.ClientID(client.ClientID))
	return client, nil
}

func (s *clientService) Update(ctx context.Context, tenantSlug string, input controlplane.ClientInput) (*repository.Client, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("admin.clients"),
		logger.Op("Update"),
		logger.TenantSlug(tenantSlug),
		logger.ClientID(input.ClientID),
	)

	if input.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}

	client, err := s.cp.UpdateClient(ctx, tenantSlug, input)
	if err != nil {
		log.Error("failed to update client", logger.Err(err))
		return nil, err
	}

	log.Info("client updated", logger.ClientID(client.ClientID))
	return client, nil
}

func (s *clientService) Delete(ctx context.Context, tenantSlug, clientID string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("admin.clients"),
		logger.Op("Delete"),
		logger.TenantSlug(tenantSlug),
		logger.ClientID(clientID),
	)

	if clientID == "" {
		return fmt.Errorf("client_id is required")
	}

	if err := s.cp.DeleteClient(ctx, tenantSlug, clientID); err != nil {
		log.Error("failed to delete client", logger.Err(err))
		return err
	}

	log.Info("client deleted", logger.ClientID(clientID))
	return nil
}
