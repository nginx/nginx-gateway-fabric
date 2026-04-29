package graph

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph/shared/secrets"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
)

const (
	SSLProtocolsKey           = "nginx.org/ssl-protocols"
	SSLCiphersKey             = "nginx.org/ssl-ciphers"
	SSLPreferServerCiphersKey = "nginx.org/ssl-prefer-server-ciphers"

	// Examples of allowed ciphers:
	//
	// Single cipher:                   AES128-GCM-SHA256
	// Cipher suite with exclusion:     HIGH:!aNULL
	// Multiple ciphers with exclusion: ECDHE-RSA-AES256-GCM-SHA384:AES128-SHA:!MD5
	// Single exclusion:                !LOW.
	sslCiphersRegx = `^!?[A-Za-z0-9-+_]+(:!?[A-Za-z0-9-+_]+)*$`
)

var (
	sslProtocolsValues           = []string{"SSLv2", "SSLv3", "TLSv1", "TLSv1.1", "TLSv1.2", "TLSv1.3"}
	sslPreferServerCiphersValues = []string{"on", "off"}
)

// Listener represents a Listener of the Gateway resource.
// For now, we only support HTTP and HTTPS listeners.
type Listener struct {
	Name string
	// GatewayName is the name of the Gateway resource this Listener belongs to.
	GatewayName types.NamespacedName
	// ListenerSetName is the name of the ListenerSet this listener comes from. Empty
	// if the listener is from a Gateway.
	ListenerSetName types.NamespacedName
	// Source holds the source of the Listener from the Gateway resource.
	Source v1.Listener
	// AllowedRouteLabelSelector is the label selector for this Listener's allowed routes, if defined.
	AllowedRouteLabelSelector labels.Selector
	// Routes holds the GRPC/HTTPRoutes attached to the Listener.
	// Only valid routes are attached.
	Routes map[RouteKey]*L7Route
	// L4Routes holds the TLSRoutes attached to the Listener.
	L4Routes map[L4RouteKey]*L4Route
	// ValidationMode holds the TLS validation configuration for the listener.
	ValidationMode v1.FrontendValidationModeType
	// CACertificateRefs holds the resolved CA certificate references for the listener.
	CACertificateRefs []v1.ObjectReference
	// ResolvedSecrets is the list of namespaced names of the Secrets resolved for this listener.
	// Only applicable for HTTPS listeners. Supports multiple certificates for SNI-based selection.
	ResolvedSecrets []types.NamespacedName
	// Conditions holds the conditions of the Listener.
	Conditions []conditions.Condition
	// SupportedKinds is the list of RouteGroupKinds allowed by the listener.
	SupportedKinds []v1.RouteGroupKind
	// Valid shows whether the Listener is valid.
	// A Listener is considered valid if NGF can generate valid NGINX configuration for it.
	Valid bool
	// Attachable shows whether Routes can attach to the Listener.
	// Listener can be invalid but still attachable.
	Attachable bool
}

func buildListeners(
	gateway *Gateway,
	sourceListeners []v1.Listener,
	gwNsName types.NamespacedName,
	listenerSetName types.NamespacedName,
) []*Listener {
	listeners := make([]*Listener, 0, len(sourceListeners))

	for _, l := range sourceListeners {
		configurator := gateway.ListenerFactory.getConfiguratorForListener(l)
		listeners = append(listeners, configurator.configure(l, gwNsName, listenerSetName, gateway))
	}

	return listeners
}

type listenerConfiguratorFactory struct {
	http, https, tls, tcp, udp, unsupportedProtocol *listenerConfigurator
}

func (f *listenerConfiguratorFactory) getConfiguratorForListener(l v1.Listener) *listenerConfigurator {
	switch l.Protocol {
	case v1.HTTPProtocolType:
		return f.http
	case v1.HTTPSProtocolType:
		return f.https
	case v1.TLSProtocolType:
		return f.tls
	case v1.TCPProtocolType:
		return f.tcp
	case v1.UDPProtocolType:
		return f.udp
	default:
		return f.unsupportedProtocol
	}
}

func newListenerConfiguratorFactory(
	gw *v1.Gateway,
	resourceResolver resolver.Resolver,
	refGrantResolver *referenceGrantResolver,
	protectedPorts ProtectedPorts,
) *listenerConfiguratorFactory {
	sharedPortConflictResolver := createPortConflictResolver()
	sharedOverlappingTLSConfigResolver := createOverlappingTLSConfigResolver()
	sharedUniqueListenerConflictResolver := uniqueListenerConflictResolver()

	return &listenerConfiguratorFactory{
		unsupportedProtocol: &listenerConfigurator{
			validators: []listenerValidator{
				func(listener v1.Listener) ([]conditions.Condition, bool) {
					valErr := field.NotSupported(
						field.NewPath("protocol"),
						listener.Protocol,
						[]string{
							string(v1.HTTPProtocolType), string(v1.HTTPSProtocolType), string(v1.TLSProtocolType),
							string(v1.TCPProtocolType), string(v1.UDPProtocolType),
						},
					)
					return conditions.NewListenerUnsupportedProtocol(valErr.Error()), false /* not attachable */
				},
			},
		},
		http: &listenerConfigurator{
			validators: []listenerValidator{
				validateListenerAllowedRouteKind,
				validateListenerLabelSelector,
				validateListenerHostname,
				createHTTPListenerValidator(protectedPorts),
			},
			conflictResolvers: []listenerConflictResolver{
				sharedPortConflictResolver,
				sharedUniqueListenerConflictResolver,
			},
		},
		https: &listenerConfigurator{
			validators: []listenerValidator{
				validateListenerAllowedRouteKind,
				validateListenerLabelSelector,
				validateListenerHostname,
				createHTTPSListenerValidator(protectedPorts),
			},
			conflictResolvers: []listenerConflictResolver{
				sharedPortConflictResolver,
				sharedOverlappingTLSConfigResolver,
				sharedUniqueListenerConflictResolver,
			},
			externalReferenceResolvers: []listenerExternalReferenceResolver{
				createExternalReferencesForTLSSecretsResolver(gw.Namespace, resourceResolver, refGrantResolver),
			},
			frontendTLSCaCertReferenceResolvers: []listenerFrontendTLSCaCertReferenceResolver{
				createFrontendTLSCaCertReferenceResolver(resourceResolver, refGrantResolver),
			},
		},
		tls: &listenerConfigurator{
			validators: []listenerValidator{
				validateListenerAllowedRouteKind,
				validateListenerLabelSelector,
				validateListenerHostname,
				validateTLSFieldOnTLSListener,
			},
			conflictResolvers: []listenerConflictResolver{
				sharedPortConflictResolver,
				sharedOverlappingTLSConfigResolver,
				sharedUniqueListenerConflictResolver,
			},
			externalReferenceResolvers:          []listenerExternalReferenceResolver{},
			frontendTLSCaCertReferenceResolvers: []listenerFrontendTLSCaCertReferenceResolver{},
		},
		tcp: &listenerConfigurator{
			validators: []listenerValidator{
				validateListenerAllowedRouteKind,
				validateListenerLabelSelector,
				createL4ListenerValidator(v1.TCPProtocolType, protectedPorts),
			},
			conflictResolvers: []listenerConflictResolver{
				sharedPortConflictResolver,
				sharedUniqueListenerConflictResolver,
			},
		},
		udp: &listenerConfigurator{
			validators: []listenerValidator{
				validateListenerAllowedRouteKind,
				validateListenerLabelSelector,
				createL4ListenerValidator(v1.UDPProtocolType, protectedPorts),
			},
			conflictResolvers: []listenerConflictResolver{
				sharedPortConflictResolver,
				sharedUniqueListenerConflictResolver,
			},
		},
	}
}

// listenerValidator validates a listener. If the listener is invalid, the validator will return appropriate conditions.
// It also returns whether the listener is attachable, which is independent of whether the listener is valid.
type listenerValidator func(v1.Listener) (conds []conditions.Condition, attachable bool)

// listenerConflictResolver resolves conflicts between listeners. In case of a conflict, the resolver will make
// the conflicting listeners invalid - i.e. it will modify the passed listener and the previously processed conflicting
// listener. It will also add appropriate conditions to the listeners.
type listenerConflictResolver func(listener *Listener)

// listenerExternalReferenceResolver resolves external references for a listener. If the reference is not resolvable,
// the resolver will make the listener invalid and add appropriate conditions.
type listenerExternalReferenceResolver func(listener *Listener)

// listenerFrontendTLSCaCertReferenceResolver resolves the CA certificate references
// for HTTPS listeners configured for frontend TLS.
// If the reference is not resolvable, the resolver will make the listener invalid and add appropriate conditions.
type listenerFrontendTLSCaCertReferenceResolver func(listener *Listener, gw *Gateway)

// listenerConfigurator is responsible for configuring a listener.
// validators, conflictResolvers, externalReferenceResolvers generate conditions for invalid fields of the listener.
// Because the Gateway status includes a status field for each listener, the messages in those conditions
// don't need to include the full path to the field (e.g. "spec.listeners[0].hostname"). They will include
// a path starting from the field of a listener (e.g. "hostname", "tls.options").
type listenerConfigurator struct {
	validators []listenerValidator
	// conflictResolvers can depend on validators - they will only be executed if all validators pass.
	conflictResolvers []listenerConflictResolver
	// externalReferenceResolvers can depend on validators - they will only be executed if all validators pass.
	externalReferenceResolvers []listenerExternalReferenceResolver
	// frontendTLSCaCertReferenceResolvers can depend on validators - they will only be executed if all validators pass.
	frontendTLSCaCertReferenceResolvers []listenerFrontendTLSCaCertReferenceResolver
}

func (c *listenerConfigurator) configure(
	listener v1.Listener,
	gwNSName,
	listenerSetName types.NamespacedName,
	gw *Gateway,
) *Listener {
	var conds []conditions.Condition

	attachable := true

	// validators might return different conditions, so we run them all.
	for _, validator := range c.validators {
		currConds, currAttachable := validator(listener)
		conds = append(conds, currConds...)

		attachable = attachable && currAttachable
	}

	valid := len(conds) == 0

	var allowedRouteSelector labels.Selector
	if selector := GetAllowedRouteLabelSelector(listener); selector != nil {
		var err error
		allowedRouteSelector, err = metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			msg := fmt.Sprintf("Invalid label selector: %s", err.Error())
			conds = append(conds, conditions.NewListenerUnsupportedValue(msg)...)
			valid = false
		}
	}

	supportedKinds := getListenerSupportedKinds(listener)

	l := &Listener{
		Name:                      string(listener.Name),
		GatewayName:               gwNSName,
		Source:                    listener,
		Conditions:                conds,
		AllowedRouteLabelSelector: allowedRouteSelector,
		Routes:                    make(map[RouteKey]*L7Route),
		L4Routes:                  make(map[L4RouteKey]*L4Route),
		Valid:                     valid,
		Attachable:                attachable,
		SupportedKinds:            supportedKinds,
		ListenerSetName:           listenerSetName,
	}

	if !l.Valid {
		return l
	}

	// resolvers might add different conditions to the listener, so we run them all.

	for _, conflictResolver := range c.conflictResolvers {
		conflictResolver(l)
	}

	for _, externalReferenceResolver := range c.externalReferenceResolvers {
		externalReferenceResolver(l)
	}

	for _, frontendTLSResolver := range c.frontendTLSCaCertReferenceResolvers {
		frontendTLSResolver(l, gw)
	}

	return l
}

func validateListenerHostname(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
	if listener.Hostname == nil {
		return nil, true
	}

	h := string(*listener.Hostname)

	if h == "" {
		return nil, true
	}

	if err := validateHostname(h); err != nil {
		path := field.NewPath("hostname")
		valErr := field.Invalid(path, listener.Hostname, err.Error())
		return conditions.NewListenerUnsupportedValue(valErr.Error()), false
	}
	return nil, true
}

// getAndValidateListenerSupportedKinds validates the route kind and returns the supported kinds for the listener.
// The supported kinds are determined based on the listener's allowedRoutes field.
// If the listener does not specify allowedRoutes, listener determines allowed routes based on its protocol.
func getAndValidateListenerSupportedKinds(listener v1.Listener) (
	[]conditions.Condition,
	[]v1.RouteGroupKind,
) {
	var conds []conditions.Condition
	var supportedKinds []v1.RouteGroupKind

	validKinds := getValidKindsForProtocol(listener)

	validProtocolRouteKind := func(kind v1.RouteGroupKind) bool {
		if kind.Group != nil && *kind.Group != v1.GroupName {
			return false
		}
		for _, k := range validKinds {
			if k.Kind == kind.Kind {
				return true
			}
		}

		return false
	}

	if listener.AllowedRoutes != nil && listener.AllowedRoutes.Kinds != nil {
		unique := make(map[string]struct{})
		supportedKinds = make([]v1.RouteGroupKind, 0)
		for _, kind := range listener.AllowedRoutes.Kinds {
			if !validProtocolRouteKind(kind) {
				group := v1.GroupName
				if kind.Group != nil {
					group = string(*kind.Group)
				}
				msg := fmt.Sprintf("Unsupported route kind for protocol %s \"%s/%s\"", listener.Protocol, group, kind.Kind)
				conds = append(conds, conditions.NewListenerInvalidRouteKinds(msg)...)
				continue
			}
			// Use group/kind as key for uniqueness
			key := string(kind.Kind)
			if kind.Group != nil {
				key = string(*kind.Group) + "/" + key
			}
			if _, exists := unique[key]; !exists {
				unique[key] = struct{}{}
				supportedKinds = append(supportedKinds, kind)
			}
		}
		return conds, supportedKinds
	}

	return conds, validKinds
}

// getValidKindsForProtocol returns the valid route kinds for a given protocol.
func getValidKindsForProtocol(listener v1.Listener) []v1.RouteGroupKind {
	switch listener.Protocol {
	case v1.HTTPProtocolType, v1.HTTPSProtocolType:
		return []v1.RouteGroupKind{
			{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
			{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
		}
	case v1.TLSProtocolType:
		if listener.TLS != nil && listener.TLS.Mode != nil && *listener.TLS.Mode == v1.TLSModePassthrough {
			return []v1.RouteGroupKind{
				{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
			}
		}
		return nil
	case v1.TCPProtocolType:
		return []v1.RouteGroupKind{
			{Kind: v1.Kind(kinds.TCPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
		}
	case v1.UDPProtocolType:
		return []v1.RouteGroupKind{
			{Kind: v1.Kind(kinds.UDPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
		}
	default:
		return nil
	}
}

func validateListenerAllowedRouteKind(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
	conds, _ = getAndValidateListenerSupportedKinds(listener)
	return conds, len(conds) == 0
}

func getListenerSupportedKinds(listener v1.Listener) []v1.RouteGroupKind {
	_, sk := getAndValidateListenerSupportedKinds(listener)
	return sk
}

func validateListenerLabelSelector(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
	if listener.AllowedRoutes != nil &&
		listener.AllowedRoutes.Namespaces != nil &&
		listener.AllowedRoutes.Namespaces.From != nil &&
		*listener.AllowedRoutes.Namespaces.From == v1.NamespacesFromSelector &&
		listener.AllowedRoutes.Namespaces.Selector == nil {
		msg := "Listener's AllowedRoutes Selector must be set when From is set to type Selector"
		return conditions.NewListenerUnsupportedValue(msg), false
	}

	return nil, true
}

func createHTTPListenerValidator(protectedPorts ProtectedPorts) listenerValidator {
	return func(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
		if err := validateListenerPort(listener.Port, protectedPorts); err != nil {
			path := field.NewPath("port")
			valErr := field.Invalid(path, listener.Port, err.Error())
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if listener.TLS != nil {
			path := field.NewPath("tls")
			valErr := field.Forbidden(path, "tls is not supported for HTTP listener")
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		return conds, true
	}
}

func validateListenerPort(port v1.PortNumber, protectedPorts ProtectedPorts) error {
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1-65535")
	}

	if portName, ok := protectedPorts[port]; ok {
		return fmt.Errorf("port is already in use as %v", portName)
	}

	return nil
}

func validateTLSFieldOnTLSListener(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
	tlspath := field.NewPath("TLS")
	if listener.TLS == nil {
		valErr := field.Required(tlspath, "tls must be defined for TLS listener")
		return conditions.NewListenerUnsupportedValue(valErr.Error()), false
	}
	if listener.TLS.Mode == nil || *listener.TLS.Mode != v1.TLSModePassthrough {
		valErr := field.Required(tlspath.Child("Mode"), "Mode must be passthrough for TLS listener")
		return conditions.NewListenerUnsupportedValue(valErr.Error()), false
	}
	return nil, true
}

func createHTTPSListenerValidator(protectedPorts ProtectedPorts) listenerValidator {
	return func(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
		if err := validateListenerPort(listener.Port, protectedPorts); err != nil {
			path := field.NewPath("port")
			valErr := field.Invalid(path, listener.Port, err.Error())
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if listener.TLS == nil {
			valErr := field.Required(field.NewPath("TLS"), "tls must be defined for HTTPS listener")
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
			return conds, true
		}

		tlsPath := field.NewPath("tls")

		if *listener.TLS.Mode != v1.TLSModeTerminate {
			valErr := field.NotSupported(
				tlsPath.Child("mode"),
				*listener.TLS.Mode,
				[]string{string(v1.TLSModeTerminate)},
			)
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if len(listener.TLS.Options) > 0 {
			conds = append(conds, validateListenerTLSOptions(listener, tlsPath)...)
		}

		if len(listener.TLS.CertificateRefs) == 0 {
			msg := "certificateRefs must be defined for TLS mode terminate"
			valErr := field.Required(tlsPath.Child("certificateRefs"), msg)
			conds = append(conds, conditions.NewListenerInvalidCertificateRefNotAccepted(valErr.Error())...)
			return conds, true
		}

		for i, certRef := range listener.TLS.CertificateRefs {
			certRefPath := tlsPath.Child("certificateRefs").Index(i)

			if certRef.Kind != nil && *certRef.Kind != "Secret" {
				path := certRefPath.Child("kind")
				valErr := field.NotSupported(path, *certRef.Kind, []string{"Secret"})
				conds = append(conds, conditions.NewListenerInvalidCertificateRefNotAccepted(valErr.Error())...)
			}

			// for Kind Secret, certRef.Group must be nil or empty
			if certRef.Group != nil && *certRef.Group != "" {
				path := certRefPath.Child("group")
				valErr := field.NotSupported(path, *certRef.Group, []string{""})
				conds = append(conds, conditions.NewListenerInvalidCertificateRefNotAccepted(valErr.Error())...)
			}
		}

		return conds, true
	}
}

func validateListenerTLSOptions(listener v1.Listener, tlsPath *field.Path) (conds []conditions.Condition) {
	supportedOptions := map[v1.AnnotationKey]bool{
		SSLProtocolsKey:           true,
		SSLCiphersKey:             true,
		SSLPreferServerCiphersKey: true,
	}

	for optionKey, optionValue := range listener.TLS.Options {
		if !supportedOptions[optionKey] {
			path := tlsPath.Child("options").Key(string(optionKey))
			valErr := field.NotSupported(path, optionKey, []string{
				SSLProtocolsKey,
				SSLCiphersKey,
				SSLPreferServerCiphersKey,
			})
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		// Validate ssl-protocols values
		if optionKey == SSLProtocolsKey {
			allowedProtocols := make(map[string]bool)

			for _, value := range sslProtocolsValues {
				allowedProtocols[value] = true
			}

			protocols := strings.Fields(string(optionValue))
			if len(protocols) == 0 {
				path := tlsPath.Child("options").Key(string(optionKey))
				valErr := field.NotSupported(path, "", sslProtocolsValues)
				conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
			}
			for _, protocol := range protocols {
				if !allowedProtocols[protocol] {
					path := tlsPath.Child("options").Key(string(optionKey))
					valErr := field.NotSupported(path, protocol, sslProtocolsValues)
					conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
				}
			}
		}

		// Validate ssl-prefer-server-ciphers values
		if optionKey == SSLPreferServerCiphersKey {
			value := string(optionValue)
			if value != "on" && value != "off" {
				path := tlsPath.Child("options").Key(string(optionKey))
				valErr := field.NotSupported(path, value, sslPreferServerCiphersValues)
				conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
			}
		}

		// Validate ssl-ciphers values
		if optionKey == SSLCiphersKey {
			value := string(optionValue)
			cipherRegex := regexp.MustCompile(sslCiphersRegx)
			if !cipherRegex.MatchString(value) {
				path := tlsPath.Child("options").Key(string(optionKey))
				valErr := field.Invalid(path, value, "invalid ssl ciphers")
				conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
			}
		}
	}

	return conds
}

// isL4Protocol checks if the protocol is a Layer 4 protocol (TCP or UDP).
func isL4Protocol(protocol v1.ProtocolType) bool {
	return protocol == v1.TCPProtocolType || protocol == v1.UDPProtocolType
}

//nolint:gocyclo // will refactor at some point
func createPortConflictResolver() listenerConflictResolver {
	const (
		secureProtocolGroup   int = 0
		insecureProtocolGroup int = 1
		l4ProtocolGroup       int = 2
	)
	protocolGroups := map[v1.ProtocolType]int{
		v1.TLSProtocolType:   secureProtocolGroup,
		v1.HTTPProtocolType:  insecureProtocolGroup,
		v1.HTTPSProtocolType: secureProtocolGroup,
		v1.TCPProtocolType:   l4ProtocolGroup,
		v1.UDPProtocolType:   l4ProtocolGroup,
	}
	conflictedPorts := make(map[v1.PortNumber]bool)
	portProtocolOwner := make(map[v1.PortNumber]int)
	listenersByPort := make(map[v1.PortNumber][]*Listener)

	format := "Multiple listeners for the same port %d specify incompatible protocols; " +
		"ensure only one protocol per port"

	formatHostname := "HTTPS and TLS listeners for the same port %d specify overlapping hostnames; " +
		"ensure no overlapping hostnames for HTTPS and TLS listeners for the same port"

	formatL4SameProtocol := "Multiple %s listeners cannot share the same port %d"

	return func(l *Listener) {
		port := l.Source.Port

		// if port is in map of conflictedPorts then we only need to set the current listener to invalid
		if conflictedPorts[port] {
			l.Valid = false

			conflictedConds := conditions.NewListenerProtocolConflict(fmt.Sprintf(format, port))
			l.Conditions = append(l.Conditions, conflictedConds...)
			return
		}

		// otherwise, we add the listener to the list of listeners for this port
		// and then check if the protocol owner for the port is different from the current listener's protocol.

		protocolGroup, ok := portProtocolOwner[port]
		if !ok {
			portProtocolOwner[port] = protocolGroups[l.Source.Protocol]
			listenersByPort[port] = append(listenersByPort[port], l)
			return
		}

		// if protocol group owner doesn't match the listener's protocol group and the conflicting listener is from a Gateway
		// (ListenerSetName is empty) we mark the port as conflicted, and invalidate all listeners we've seen for this port.
		// This is just to satisfy a specific listener conflict case. However, if the conflicting listener
		// is from a ListenerSet,
		// this means we are currently merging listeners from a ListenerSet onto the Gateway, and we can only mark the current
		// listener as invalid, allowing the existing listener(s) on the Gateway (native to the Gateway
		// or already merged from a ListenerSet)
		// to stay valid.
		if protocolGroup != protocolGroups[l.Source.Protocol] {
			if l.ListenerSetName.Name == "" {
				conflictedPorts[port] = true
				for _, listener := range listenersByPort[port] {
					listener.Valid = false
					conflictedConds := conditions.NewListenerProtocolConflict(fmt.Sprintf(format, port))
					listener.Conditions = append(listener.Conditions, conflictedConds...)
				}
			}

			l.Valid = false
			conflictedConds := conditions.NewListenerProtocolConflict(fmt.Sprintf(format, port))
			l.Conditions = append(l.Conditions, conflictedConds...)
		} else {
			foundConflict := false
			for _, listener := range listenersByPort[port] {
				if isL4Protocol(l.Source.Protocol) &&
					listener.Source.Protocol == l.Source.Protocol {
					// Similar to the case above, if the conflicting listener is from a ListenerSet,
					// we only mark the current listener as invalid.
					if l.ListenerSetName.Name == "" {
						listener.Valid = false
						conflictedConds := conditions.NewListenerProtocolConflict(
							fmt.Sprintf(formatL4SameProtocol, l.Source.Protocol, port))
						listener.Conditions = append(listener.Conditions, conflictedConds...)
					}
					foundConflict = true
				}
				if listener.Source.Protocol != l.Source.Protocol &&
					!isL4Protocol(listener.Source.Protocol) && !isL4Protocol(l.Source.Protocol) &&
					haveOverlap(l.Source.Hostname, listener.Source.Hostname) {
					// Similar to the case above, if the conflicting listener is from a ListenerSet,
					// we only mark the current listener as invalid.
					if l.ListenerSetName.Name == "" {
						listener.Valid = false
						conflictedConds := conditions.NewListenerHostnameConflict(fmt.Sprintf(formatHostname, port))
						listener.Conditions = append(listener.Conditions, conflictedConds...)
					}
					foundConflict = true
				}
			}

			if foundConflict {
				l.Valid = false
				if isL4Protocol(l.Source.Protocol) {
					conflictedConds := conditions.NewListenerProtocolConflict(
						fmt.Sprintf(formatL4SameProtocol, l.Source.Protocol, port))
					l.Conditions = append(l.Conditions, conflictedConds...)
				} else {
					conflictedConds := conditions.NewListenerHostnameConflict(fmt.Sprintf(formatHostname, port))
					l.Conditions = append(l.Conditions, conflictedConds...)
				}
			}
		}

		listenersByPort[port] = append(listenersByPort[port], l)
	}
}

func uniqueListenerConflictResolver() listenerConflictResolver {
	type listenerKey struct {
		protocol v1.ProtocolType
		hostname string
		port     v1.PortNumber
	}

	existingKeys := make(map[listenerKey]struct{})

	return func(l *Listener) {
		// Just to satisfy some conformance test condition expectations, if the listener is already invalid we skip the check
		// Otherwise, these conditions might overlap with the conditions generated by the port conflict resolver
		if !l.Valid {
			return
		}

		hostname := ""
		if l.Source.Hostname != nil {
			hostname = string(*l.Source.Hostname)
		}

		key := listenerKey{
			port:     l.Source.Port,
			protocol: l.Source.Protocol,
			hostname: hostname,
		}

		if _, exists := existingKeys[key]; exists {
			msg := fmt.Sprintf("Multiple listeners with the same port %d and protocol %s have overlapping hostnames",
				key.port, key.protocol)
			l.Valid = false
			l.Conditions = append(l.Conditions, conditions.NewListenerHostnameConflict(msg)...)
			return
		}

		// Only add valid listener to the map for future conflict detection
		existingKeys[key] = struct{}{}
	}
}

type certRefError struct {
	msg             string
	refNotPermitted bool
}

//nolint:gocyclo // will refactor at some point
func createExternalReferencesForTLSSecretsResolver(
	gwNs string,
	resourceResolver resolver.Resolver,
	refGrantResolver *referenceGrantResolver,
) listenerExternalReferenceResolver {
	return func(l *Listener) {
		if !l.Valid {
			return
		}

		var certRefErrors []certRefError

		for i, certRef := range l.Source.TLS.CertificateRefs {
			var certRefNs string
			if l.ListenerSetName.Name != "" {
				certRefNs = l.ListenerSetName.Namespace
			} else {
				certRefNs = gwNs
			}

			if certRef.Namespace != nil {
				certRefNs = string(*certRef.Namespace)
			}

			certRefNsName := types.NamespacedName{
				Namespace: certRefNs,
				Name:      string(certRef.Name),
			}

			msg := fmt.Sprintf("Certificate ref to secret %s not permitted by any ReferenceGrant", certRefNsName)
			if l.ListenerSetName.Name != "" && certRefNs != l.ListenerSetName.Namespace {
				if !refGrantResolver.refAllowed(toSecret(certRefNsName), fromListenerSet(l.ListenerSetName.Namespace)) {
					certRefErrors = append(certRefErrors, certRefError{msg: msg, refNotPermitted: true})
					continue
				}
			} else if certRefNs != gwNs {
				if !refGrantResolver.refAllowed(toSecret(certRefNsName), fromGateway(gwNs)) {
					certRefErrors = append(certRefErrors, certRefError{msg: msg, refNotPermitted: true})
					continue
				}
			}

			if err := resourceResolver.Resolve(resolver.ResourceTypeSecret, certRefNsName); err != nil {
				path := field.NewPath("tls", "certificateRefs").Index(i)
				valErr := field.Invalid(path, certRefNsName, err.Error())
				certRefErrors = append(certRefErrors, certRefError{msg: valErr.Error()})
				continue
			}

			l.ResolvedSecrets = append(l.ResolvedSecrets, certRefNsName)
		}

		if len(certRefErrors) > 0 {
			// Aggregate all error messages into a single condition so that
			// deduplication (which keeps one condition per Type) does not drop any information.
			var allMsgs []string
			hasInvalidRef := false
			for _, certErr := range certRefErrors {
				allMsgs = append(allMsgs, certErr.msg)
				if !certErr.refNotPermitted {
					hasInvalidRef = true
				}
			}

			// Pick the most representative reason.
			// If any cert ref is invalid, use InvalidCertificateRef (the broader reason).
			// If all errors are ref-not-permitted, use RefNotPermitted.
			reason := string(v1.ListenerReasonRefNotPermitted)
			if hasInvalidRef {
				reason = string(v1.ListenerReasonInvalidCertificateRef)
			}

			msg := strings.Join(allMsgs, "; ")

			if len(l.ResolvedSecrets) == 0 {
				// All certs are invalid; the listener is not valid.
				l.Valid = false
				l.Conditions = append(
					l.Conditions,
					conditions.NewListenerAllInvalidCertificateRefs(msg, reason)...,
				)
			} else {
				// Some certs are valid, some are not.
				// Keep the listener valid so valid certs are still configured,
				// but set ResolvedRefs to false.
				l.Conditions = append(
					l.Conditions,
					conditions.NewListenerUnresolvedCertificateRef(msg, reason),
				)
			}
		}
	}
}

func createFrontendTLSCaCertReferenceResolver(
	resourceResolver resolver.Resolver,
	refGrantResolver *referenceGrantResolver,
) listenerFrontendTLSCaCertReferenceResolver {
	return func(l *Listener, gw *Gateway) {
		if gw.Source.Spec.TLS == nil || gw.Source.Spec.TLS.Frontend == nil {
			return
		}

		if l.Source.TLS == nil || (l.Source.TLS.Mode != nil && *l.Source.TLS.Mode != v1.TLSModeTerminate) {
			return
		}

		frontend := gw.Source.Spec.TLS.Frontend

		var caCertRefs []v1.ObjectReference
		var validationMode v1.FrontendValidationModeType
		var fieldPath *field.Path
		perPortMatch := false

		for i, port := range frontend.PerPort {
			if port.TLS.Validation == nil || len(port.TLS.Validation.CACertificateRefs) == 0 {
				continue
			}
			if port.Port == l.Source.Port {
				caCertRefs = port.TLS.Validation.CACertificateRefs
				validationMode = port.TLS.Validation.Mode
				fieldPath = field.NewPath("spec", "tls", "frontend", "perPort").Index(i).Child("tls", "validation")
				perPortMatch = true
				break
			}
		}

		if !perPortMatch && frontend.Default.Validation != nil &&
			len(frontend.Default.Validation.CACertificateRefs) > 0 {
			caCertRefs = frontend.Default.Validation.CACertificateRefs
			validationMode = frontend.Default.Validation.Mode
			fieldPath = field.NewPath("spec", "tls", "frontend", "default", "validation")
		}

		conds := validateFrontendTLS(
			gw,
			l,
			fieldPath,
			resourceResolver,
			refGrantResolver,
			caCertRefs,
		)
		l.Conditions = append(l.Conditions, conds...)
		l.ValidationMode = validationMode
		l.CACertificateRefs = caCertRefs

		if l.ValidationMode == v1.AllowInsecureFallback {
			msg := "Validation Mode: AllowInsecureFallback is set for at least one listener"
			gw.Conditions = append(gw.Conditions, conditions.NewGatewayInsecureFrontendValidationMode(msg))
		}
	}
}

// GetAllowedRouteLabelSelector returns a listener's AllowedRoutes label selector if it exists.
func GetAllowedRouteLabelSelector(l v1.Listener) *metav1.LabelSelector {
	if l.AllowedRoutes != nil && l.AllowedRoutes.Namespaces != nil {
		if *l.AllowedRoutes.Namespaces.From == v1.NamespacesFromSelector &&
			l.AllowedRoutes.Namespaces.Selector != nil {
			return l.AllowedRoutes.Namespaces.Selector
		}
	}

	return nil
}

// matchesWildcard checks if hostname2 matches the wildcard pattern of hostname1.
func matchesWildcard(hostname1, hostname2 string) bool {
	mw := func(h1, h2 string) bool {
		if strings.HasPrefix(h1, "*.") {
			// Remove the "*" from h1
			suffix := h1[1:]
			// Check if h2 ends with the suffix
			return strings.HasSuffix(h2, suffix)
		}
		return false
	}
	return mw(hostname1, hostname2) || mw(hostname2, hostname1)
}

// haveOverlap checks for overlap between two hostnames.
func haveOverlap(hostname1, hostname2 *v1.Hostname) bool {
	// Check if hostname1 matches wildcard pattern of hostname2 or vice versa
	if hostname1 == nil || hostname2 == nil {
		return true
	}
	h1, h2 := string(*hostname1), string(*hostname2)

	if h1 == h2 {
		return true
	}
	return matchesWildcard(h1, h2)
}

func createL4ListenerValidator(protocol v1.ProtocolType, protectedPorts ProtectedPorts) listenerValidator {
	return func(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
		if err := validateListenerPort(listener.Port, protectedPorts); err != nil {
			path := field.NewPath("port")
			valErr := field.Invalid(path, listener.Port, err.Error())
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if listener.TLS != nil {
			path := field.NewPath("tls")
			valErr := field.Forbidden(path, fmt.Sprintf("tls is not supported for %s listener", protocol))
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if listener.Hostname != nil {
			path := field.NewPath("hostname")
			valErr := field.Forbidden(path, fmt.Sprintf("hostname is not supported for %s listener", protocol))
			conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error())...)
		}

		return conds, true
	}
}

func createOverlappingTLSConfigResolver() listenerConflictResolver {
	listenersByPort := make(map[v1.PortNumber][]*Listener)

	return func(l *Listener) {
		port := l.Source.Port

		// Only check TLS-enabled listeners (HTTPS/TLS)
		if l.Source.Protocol != v1.HTTPSProtocolType && l.Source.Protocol != v1.TLSProtocolType {
			return
		}

		// Check for overlaps with existing listeners on this port
		for _, existingListener := range listenersByPort[port] {
			// Only check against other TLS-enabled listeners
			if existingListener.Source.Protocol != v1.HTTPSProtocolType &&
				existingListener.Source.Protocol != v1.TLSProtocolType {
				continue
			}

			// Check for hostname overlap
			if haveOverlap(l.Source.Hostname, existingListener.Source.Hostname) {
				// Set condition on both listeners
				cond := conditions.NewListenerOverlappingTLSConfig(
					v1.ListenerReasonOverlappingHostnames,
					conditions.ListenerMessageOverlappingHostnames,
				)
				l.Conditions = append(l.Conditions, cond)
				existingListener.Conditions = append(existingListener.Conditions, cond)
			}
		}

		listenersByPort[port] = append(listenersByPort[port], l)
	}
}

// validateFrontendTLS validates and resolves the CA certificate references
// for a listener configured with frontend TLS.
// Returns conditions related to invalid CA certificate references.
func validateFrontendTLS(
	gw *Gateway,
	listener *Listener,
	path *field.Path,
	resourceResolver resolver.Resolver,
	refGrantResolver *referenceGrantResolver,
	caCertRefs []v1.ObjectReference,
) []conditions.Condition {
	if gw.Source.Spec.TLS == nil || gw.Source.Spec.TLS.Frontend == nil {
		return []conditions.Condition{}
	}

	var conds []conditions.Condition
	refNotPermittedCount := 0
	allowedKinds := []string{kinds.Secret, kinds.ConfigMap}

	for _, cert := range caCertRefs {
		if kindOrGroupCond := validateObjectRefKindAndGroup(
			cert,
			path,
			allowedKinds,
		); kindOrGroupCond != (conditions.Condition{}) {
			conds = append(conds, kindOrGroupCond)
			continue
		}

		certNsName := getFrontendTLSCertRefNsName(cert, gw.Source)
		resourceType := getFrontendTLSCertResourceType(cert.Kind)

		if refNotPermittedCond := resolveCrossNamespaceRefGrant(
			cert,
			certNsName,
			gw.Source.Namespace,
			refGrantResolver,
		); refNotPermittedCond != (conditions.Condition{}) {
			gw.Conditions = append(gw.Conditions, refNotPermittedCond)
			refNotPermittedCount++
			continue
		}

		if err := resourceResolver.Resolve(
			resourceType,
			*certNsName,
			resolver.WithExpectedSecretKey(secrets.CAKey),
		); err != nil {
			valErr := field.Invalid(path.Child("caCertificateRefs"), certNsName, err.Error())
			msg := helpers.CapitalizeString(valErr.Error())
			conds = append(conds, conditions.NewListenerInvalidCaCertificateRef(msg))
			continue
		}
	}

	totalConds := len(conds) + refNotPermittedCount
	if refNotPermittedCount > 0 {
		msg := "Frontend TLS CA certificate refs are not permitted by any ReferenceGrant"
		conds = append(conds, conditions.NewListenerUnresolvedCertificateRef(
			msg,
			string(v1.ListenerReasonRefNotPermitted),
		))
	}

	if totalConds > 0 && totalConds == len(caCertRefs) {
		msg := "All frontend TLS CA certificate refs are invalid for this listener"
		conds = append(conds, conditions.NewListenerInvalidNoValidCACertificate(msg)...)
		listener.Valid = false
		return conds
	}

	return conds
}

// validateObjectRefKindAndGroup checks if the ObjectReference has an allowed Kind and Group.
func validateObjectRefKindAndGroup(
	ref v1.ObjectReference,
	path *field.Path,
	allowedKinds []string,
) conditions.Condition {
	if !slices.Contains(allowedKinds, string(ref.Kind)) {
		valErr := field.NotSupported(path, ref.Kind, allowedKinds)
		msg := helpers.CapitalizeString(valErr.Error())
		return conditions.NewListenerInvalidCaCertificateKind(msg)
	}

	if ref.Group != "" && ref.Group != "core" {
		valErr := field.NotSupported(path, ref.Group, []string{"core", ""})
		msg := helpers.CapitalizeString(valErr.Error())
		return conditions.NewListenerInvalidCaCertificateKind(msg)
	}

	return conditions.Condition{}
}

// getFrontendTLSCertResourceType returns the resource type for a given kind.
func getFrontendTLSCertResourceType(kind v1.Kind) resolver.ResourceType {
	switch kind {
	case kinds.Secret:
		return resolver.ResourceTypeSecret
	case kinds.ConfigMap:
		return resolver.ResourceTypeConfigMap
	default:
		return ""
	}
}

// resolveCrossNamespaceRefGrant checks if a cross-namespace reference is allowed by any ReferenceGrant.
// Checks for both Secret and ConfigMap references.
func resolveCrossNamespaceRefGrant(
	ref v1.ObjectReference,
	nsName *types.NamespacedName,
	gwNs string,
	refGrantResolver *referenceGrantResolver,
) conditions.Condition {
	if nsName.Namespace == gwNs {
		return conditions.Condition{}
	}

	switch ref.Kind {
	case kinds.Secret:
		if !refGrantResolver.refAllowed(toSecret(*nsName), fromGateway(gwNs)) {
			msg := fmt.Sprintf("secret ref %s not permitted by any ReferenceGrant", nsName)
			return conditions.NewGatewayRefNotPermitted(msg)
		}
	case kinds.ConfigMap:
		if !refGrantResolver.refAllowed(toConfigMap(*nsName), fromGateway(gwNs)) {
			msg := fmt.Sprintf("configmap ref %s not permitted by any ReferenceGrant", nsName)
			return conditions.NewGatewayRefNotPermitted(msg)
		}
	}
	return conditions.Condition{}
}

// getFrontendTLSCertRefNsName returns a NamespacedName
// of the Secret or ConfigMap referenced by the Gateway for frontend TLS.
func getFrontendTLSCertRefNsName(
	cert v1.ObjectReference,
	gw *v1.Gateway,
) *types.NamespacedName {
	caRefNs := gw.Namespace
	if cert.Namespace != nil {
		caRefNs = string(*cert.Namespace)
	}
	caCertNsName := &types.NamespacedName{
		Namespace: caRefNs,
		Name:      string(cert.Name),
	}
	return caCertNsName
}
