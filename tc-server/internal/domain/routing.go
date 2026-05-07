package domain

import (
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func EndpointCanHandle(endpoint Endpoint, targetCapability string) bool {
	return capabilityMatches(endpoint.Capabilities, targetCapability)
}

func MessageTargetsEndpoint(message Message, endpointRef string) bool {
	return message.TargetEndpointRef == "" || message.TargetEndpointRef == endpointRef
}

func MessageRoutableToEndpoint(message Message, endpoint Endpoint) bool {
	return MessageTargetsEndpoint(message, endpoint.EndpointRef) &&
		EndpointCanHandle(endpoint, message.TargetCapability)
}

func MessageDependenciesCompleted(message Message, states map[string]string) bool {
	for _, ref := range message.DependsOnMessageRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if states[ref] != MessageStateCompleted {
			return false
		}
	}
	return true
}

func capabilityMatches(capabilities map[string]contracts.Capability, targetCapability string) bool {
	targetCapability = strings.TrimSpace(targetCapability)
	if targetCapability == "" {
		return false
	}
	if _, ok := capabilities[targetCapability]; ok {
		return true
	}
	for name := range capabilities {
		name = strings.TrimSpace(name)
		if !strings.HasSuffix(name, ".*") {
			continue
		}
		prefix := strings.TrimSuffix(name, "*")
		if strings.HasPrefix(targetCapability, prefix) {
			return true
		}
	}
	return false
}
