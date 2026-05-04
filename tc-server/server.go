package tcserver

import (
	"errors"
	"net/http"

	"github.com/nangman-infra/touch-connect/tc-server/internal/adapters/api"
	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nangman-infra/touch-connect/tc-server/internal/infrastructure/memory"
	sqlitestore "github.com/nangman-infra/touch-connect/tc-server/internal/infrastructure/sqlite"
)

type Server struct {
	service *application.Service
	handler http.Handler
}

type Settings = application.Settings

func DefaultSettings() Settings {
	return application.DefaultSettings()
}

func NewInMemoryServer() *Server {
	server, err := NewInMemoryServerWithSettings(application.DefaultSettings())
	if err != nil {
		panic(err)
	}
	return server
}

func NewInMemoryServerWithSettings(settings application.Settings) (*Server, error) {
	store := memory.NewStore()
	return NewServerWithStore(store, settings)
}

func NewSQLiteServer(path string, settings application.Settings) (*Server, error) {
	store, err := sqlitestore.NewStore(path)
	if err != nil {
		return nil, err
	}
	return NewServerWithStore(store, settings)
}

func NewServerWithStore(store application.Store, settings application.Settings) (*Server, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}
	return NewServerWithPorts(store, store, store, store, store, store, store, store, settings)
}

func NewServerWithPorts(endpoints application.EndpointRegistry, messages application.MessageLedger, processing application.ProcessingLedger, readbacks application.ReadbackLedger, artifacts application.ArtifactLedger, governance application.GovernanceLedger, refs application.RefAllocator, projections application.ProjectionReader, settings application.Settings) (*Server, error) {
	return NewServerWithPortsAndDeliveryAdapter(endpoints, messages, processing, readbacks, artifacts, governance, nil, refs, projections, settings)
}

func NewServerWithPortsAndDeliveryAdapter(endpoints application.EndpointRegistry, messages application.MessageLedger, processing application.ProcessingLedger, readbacks application.ReadbackLedger, artifacts application.ArtifactLedger, governance application.GovernanceLedger, delivery application.DeliveryAdapter, refs application.RefAllocator, projections application.ProjectionReader, settings application.Settings) (*Server, error) {
	service, err := application.NewServiceWithDeliveryAdapter(endpoints, messages, processing, readbacks, artifacts, governance, delivery, refs, projections, settings)
	if err != nil {
		return nil, err
	}
	return &Server{
		service: service,
		handler: api.NewHandler(service),
	}, nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) Snapshot() domain.Snapshot {
	return s.service.Snapshot()
}

func (s *Server) ReconcileExpiredClaims() int {
	return s.service.ReconcileExpiredClaims()
}
