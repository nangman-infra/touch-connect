package application

import "github.com/nangman-infra/touch-connect/tc-server/internal/domain"

type DeliveryAdapter interface {
	PublishAcceptedMessage(message domain.Message) (DeliveryReceipt, error)
	FetchNextDelivery(request DeliveryFetchRequest) (DeliveryRecord, bool, error)
	AckDelivery(deliveryRef string) error
	NakDelivery(deliveryRef string, reason string) error
}

type DeliveryFetchRequest struct {
	EndpointRef  string
	Capabilities []string
}

type DeliveryRecord struct {
	DeliveryRef string
	MessageRef  string
	Subject     string
	Metadata    map[string]string
}

type DeliveryReceipt struct {
	DeliveryRef string
	Metadata    map[string]string
}
