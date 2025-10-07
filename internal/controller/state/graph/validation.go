package graph

import (
	"errors"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

func validateHostname(hostname string) error {
	if hostname == "" {
		return errors.New("cannot be empty string")
	}

	if strings.HasPrefix(hostname, "*.") {
		msgs := validation.IsWildcardDNS1123Subdomain(hostname)
		if len(msgs) > 0 {
			combined := strings.Join(msgs, ",")
			return errors.New(combined)
		}
		return nil
	}

	msgs := validation.IsDNS1123Subdomain(hostname)
	if len(msgs) > 0 {
		combined := strings.Join(msgs, ",")
		return errors.New(combined)
	}

	return nil
}

// RuleWithUnsupportedFields defines an interface for rules with unsupported fields.
type RuleWithUnsupportedFields interface {
	GetName() *v1.SectionName
	GetTimeouts() *v1.HTTPRouteTimeouts
	GetRetry() *v1.HTTPRouteRetry
	GetSessionPersistence() *v1.SessionPersistence
	GetSupportedFields() []string
}
