package main

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	// Regex from: https://github.com/kubernetes-sigs/gateway-api/blob/v1.2.1/apis/v1/shared_types.go#L660
	controllerNameRegex = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/[A-Za-z0-9\/\-._~%!$&'()*+,;=:]+$` //nolint:lll
)

func validateGatewayControllerName(value string) error {
	if len(value) == 0 {
		return errors.New("must be set")
	}

	fields := strings.Split(value, "/")
	l := len(fields)
	if l < 2 {
		return errors.New("invalid format; must be DOMAIN/PATH")
	}

	if fields[0] != domain {
		return fmt.Errorf("invalid domain: %s; domain must be: %s", fields[0], domain)
	}

	re := regexp.MustCompile(controllerNameRegex)
	if !re.MatchString(value) {
		return fmt.Errorf("invalid gateway controller name: %s; expected format is DOMAIN/PATH", value)
	}

	return nil
}

func validateResourceName(value string) error {
	if len(value) == 0 {
		return errors.New("must be set")
	}

	// used by Kubernetes to validate resource names
	messages := validation.IsDNS1123Subdomain(value)
	if len(messages) > 0 {
		msg := strings.Join(messages, "; ")
		return fmt.Errorf("invalid format: %s", msg)
	}

	return nil
}

func validateNamespaceName(value string) error {
	// used by Kubernetes to validate resource namespace names
	messages := validation.IsDNS1123Label(value)
	if len(messages) > 0 {
		msg := strings.Join(messages, "; ")
		return fmt.Errorf("invalid format: %s", msg)
	}

	return nil
}

func parseNamespacedResourceName(value string) (types.NamespacedName, error) {
	if value == "" {
		return types.NamespacedName{}, errors.New("must be set")
	}

	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return types.NamespacedName{}, errors.New("invalid format; must be NAMESPACE/NAME")
	}

	if err := validateNamespaceName(parts[0]); err != nil {
		return types.NamespacedName{}, fmt.Errorf("invalid namespace name: %w", err)
	}
	if err := validateResourceName(parts[1]); err != nil {
		return types.NamespacedName{}, fmt.Errorf("invalid resource name: %w", err)
	}

	return types.NamespacedName{
		Namespace: parts[0],
		Name:      parts[1],
	}, nil
}

func validateQualifiedName(name string) error {
	if len(name) == 0 {
		return errors.New("must be set")
	}

	messages := validation.IsQualifiedName(name)
	if len(messages) > 0 {
		msg := strings.Join(messages, "; ")
		return fmt.Errorf("invalid format: %s", msg)
	}

	return nil
}

func validateIP(ip string) error {
	if ip == "" {
		return errors.New("IP address must be set")
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("%q must be a valid IP address", ip)
	}

	return nil
}

// validateEndpoint validates an endpoint, which is <host>:<port> where host is either a hostname or an IP address.
func validateEndpoint(endpoint string) error {
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("%q must be in the format <host>:<port>: %w", endpoint, err)
	}

	portVal, err := strconv.ParseInt(port, 10, 16)
	if err != nil {
		return fmt.Errorf("port must be a valid number: %w", err)
	}

	if portVal < 1 || portVal > 65535 {
		return fmt.Errorf("port outside of valid port range [1 - 65535]: %v", port)
	}

	if err := validateIP(host); err == nil {
		return nil
	}

	if errs := validation.IsDNS1123Subdomain(host); len(errs) == 0 {
		return nil
	}

	// we don't know if the user intended to use a hostname or an IP address,
	// so we return a generic error message
	return fmt.Errorf("%q must be in the format <host>:<port>", endpoint)
}

func validateEndpointOptionalPort(value string) error {
	if len(value) == 0 {
		return errors.New("must be set")
	}

	// This function assumes a port exists. If it doesn't, ignore those errors. Any errors with the endpoint
	// will be caught by further validation.
	host, port, err := net.SplitHostPort(value)
	if err != nil &&
		(!strings.Contains(err.Error(), "missing port") && !strings.Contains(err.Error(), "too many colons")) {
		return fmt.Errorf("error splitting %q into host and port: %w", value, err)
	}

	if port != "" {
		portVal, err := strconv.ParseInt(port, 10, 16)
		if err != nil {
			return fmt.Errorf("port must be a valid number: %w", err)
		}

		if portVal < 1 || portVal > 65535 {
			return fmt.Errorf("port outside of valid port range [1 - 65535]: %v", port)
		}
	}

	if host == "" {
		host = value
	}

	if err := validateIP(host); err == nil {
		return nil
	}

	if errs := validation.IsDNS1123Subdomain(host); len(errs) == 0 {
		return nil
	}

	// we don't know if the user intended to use a hostname or an IP address,
	// so we return a generic error message
	return fmt.Errorf("%q must be a domain name or IP address with optional port", value)
}

// validatePort makes sure a given port is inside the valid port range for its usage.
func validatePort(port int) error {
	if port < 1024 || port > 65535 {
		return fmt.Errorf("port outside of valid port range [1024 - 65535]: %v", port)
	}
	return nil
}

// ensureNoPortCollisions checks if the same port has been defined multiple times.
func ensureNoPortCollisions(ports ...int) error {
	seen := make(map[int]struct{})

	for _, port := range ports {
		if _, ok := seen[port]; ok {
			return fmt.Errorf("port %d has been defined multiple times", port)
		}
		seen[port] = struct{}{}
	}

	return nil
}

// validateCopyArgs ensures that arguments to the initialize command are set.
func validateCopyArgs(srcFiles []string, destDirs []string) error {
	if len(srcFiles) != len(destDirs) {
		return errors.New("source and destination must have the same number of elements")
	}
	if len(srcFiles) == 0 {
		return errors.New("source must not be empty")
	}
	if len(destDirs) == 0 {
		return errors.New("destination must not be empty")
	}

	return nil
}
