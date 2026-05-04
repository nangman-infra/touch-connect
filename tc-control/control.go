package tccontrol

import (
	"net/http"

	"github.com/nangman-infra/touch-connect/tc-control/internal/api"
	"github.com/nangman-infra/touch-connect/tc-control/internal/application"
	"github.com/nangman-infra/touch-connect/tc-control/internal/serverapi"
)

type Server struct {
	service *application.Service
	handler http.Handler
}

func New(serverURL string, httpClient *http.Client, version string) (*Server, error) {
	client := serverapi.NewClient(serverURL, httpClient)
	service, err := application.NewService(client, version)
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
