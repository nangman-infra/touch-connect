package tcserver

import (
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
	service, err := application.NewService(store, settings)
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
