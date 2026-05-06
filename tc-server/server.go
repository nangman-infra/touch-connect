package tcserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nangman-infra/touch-connect/tc-server/internal/adapters/api"
	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nangman-infra/touch-connect/tc-server/internal/infrastructure/memory"
	sqlitestore "github.com/nangman-infra/touch-connect/tc-server/internal/infrastructure/sqlite"
)

type Server struct {
	service  *application.Service
	handler  http.Handler
	settings application.Settings
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
	return NewServerWithPorts(application.PortsFromStore(store), settings)
}

func NewServerWithPorts(ports application.ServicePorts, settings application.Settings) (*Server, error) {
	return NewServerWithPortsAndDeliveryAdapter(ports, nil, settings)
}

func NewServerWithPortsAndDeliveryAdapter(ports application.ServicePorts, delivery application.DeliveryAdapter, settings application.Settings) (*Server, error) {
	service, err := application.NewServiceWithDeliveryAdapter(ports, delivery, settings)
	if err != nil {
		return nil, err
	}
	return &Server{
		service:  service,
		handler:  api.NewHandler(service),
		settings: settings,
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

func (s *Server) StartBackgroundReconcile(ctx context.Context, interval time.Duration) <-chan struct{} {
	if interval <= 0 {
		interval = s.settings.AttemptLeaseDuration / 2
	}
	if interval <= 0 {
		interval = time.Second
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.ReconcileExpiredClaims()
			}
		}
	}()
	return done
}
