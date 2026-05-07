package sqlite

import (
	"database/sql"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Store) SaveEndpoint(endpoint domain.Endpoint) error {
	body, err := encode(endpoint)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO endpoints(endpoint_ref, body) VALUES(?, ?)
ON CONFLICT(endpoint_ref) DO UPDATE SET body = excluded.body`, endpoint.EndpointRef, body)
	return err
}

func (s *Store) GetEndpoint(endpointRef string) (domain.Endpoint, bool) {
	var body string
	err := s.db.QueryRow(`SELECT body FROM endpoints WHERE endpoint_ref = ?`, endpointRef).Scan(&body)
	if err == sql.ErrNoRows {
		return domain.Endpoint{}, false
	}
	if err != nil {
		return domain.Endpoint{}, false
	}
	endpoint, err := decode[domain.Endpoint](body)
	return endpoint, err == nil
}

func (s *Store) UpdateEndpoint(endpoint domain.Endpoint) error {
	body, err := encode(endpoint)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`UPDATE endpoints SET body = ? WHERE endpoint_ref = ?`, body, endpoint.EndpointRef)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domain.ErrEndpointNotFound
	}
	return nil
}

func (s *Store) UpdateCapabilities(endpointRef string, capabilities map[string]contracts.Capability) (domain.Endpoint, error) {
	endpoint, ok := s.GetEndpoint(endpointRef)
	if !ok {
		return domain.Endpoint{}, domain.ErrEndpointNotFound
	}
	endpoint.Capabilities = capabilities
	if err := s.UpdateEndpoint(endpoint); err != nil {
		return domain.Endpoint{}, err
	}
	return endpoint, nil
}

func (s *Store) CapabilityEndpoints(capability string) []domain.Endpoint {
	rows, err := s.db.Query(`SELECT body FROM endpoints ORDER BY endpoint_ref`)
	if err != nil {
		return nil
	}
	endpoints, err := decodeRows[domain.Endpoint](rows)
	if err != nil {
		return nil
	}
	filtered := make([]domain.Endpoint, 0)
	for _, endpoint := range endpoints {
		if domain.EndpointCanHandle(endpoint, capability) {
			filtered = append(filtered, endpoint)
		}
	}
	return filtered
}
