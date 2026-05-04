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
	refs, ok := store.(application.RefAllocator)
	if !ok {
		return nil, errors.New("store must provide ref allocator")
	}
	projections, ok := store.(application.ProjectionReader)
	if !ok {
		return nil, errors.New("store must provide projection reader")
	}
	endpoints, ok := store.(application.EndpointRegistry)
	if !ok {
		return nil, errors.New("store must provide endpoint registry")
	}
	messages, ok := store.(application.MessageLedger)
	if !ok {
		return nil, errors.New("store must provide message ledger")
	}
	return NewServerWithPorts(store, endpoints, messages, refs, projections, settings)
}

func NewServerWithPorts(store application.Store, endpoints application.EndpointRegistry, messages application.MessageLedger, refs application.RefAllocator, projections application.ProjectionReader, settings application.Settings) (*Server, error) {
	service, err := application.NewService(store, endpoints, messages, refs, projections, settings)
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
