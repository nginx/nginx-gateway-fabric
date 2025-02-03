package graph

import (
	"errors"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	staticConds "github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
)

// Listener represents a Listener of the Gateway resource.
// For now, we only support HTTP and HTTPS listeners.
type Listener struct {
	Name string
	// Source holds the source of the Listener from the Gateway resource.
	Source v1.Listener
	// Routes holds the GRPC/HTTPRoutes attached to the Listener.
	// Only valid routes are attached.
	Routes map[RouteKey]*L7Route
	// L4Routes holds the TLSRoutes attached to the Listener.
	L4Routes map[L4RouteKey]*L4Route
	// AllowedRouteLabelSelector is the label selector for this Listener's allowed routes, if defined.
	AllowedRouteLabelSelector labels.Selector
	// ResolvedSecret is the namespaced name of the Secret resolved for this listener.
	// Only applicable for HTTPS listeners.
	ResolvedSecret *types.NamespacedName
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
	gw *v1.Gateway,
	secretResolver *secretResolver,
	refGrantResolver *referenceGrantResolver,
	protectedPorts ProtectedPorts,
) []*Listener {
	listeners := make([]*Listener, 0, len(gw.Spec.Listeners))

	listenerFactory := newListenerConfiguratorFactory(gw, secretResolver, refGrantResolver, protectedPorts)

	for _, gl := range gw.Spec.Listeners {
		configurator := listenerFactory.getConfiguratorForListener(gl)
		listeners = append(listeners, configurator.configure(gl))
	}

	return listeners
}

type listenerConfiguratorFactory struct {
	http, https, tls, unsupportedProtocol *listenerConfigurator
}

func (f *listenerConfiguratorFactory) getConfiguratorForListener(l v1.Listener) *listenerConfigurator {
	switch l.Protocol {
	case v1.HTTPProtocolType:
		return f.http
	case v1.HTTPSProtocolType:
		return f.https
	case v1.TLSProtocolType:
		return f.tls
	default:
		return f.unsupportedProtocol
	}
}

func newListenerConfiguratorFactory(
	gw *v1.Gateway,
	secretResolver *secretResolver,
	refGrantResolver *referenceGrantResolver,
	protectedPorts ProtectedPorts,
) *listenerConfiguratorFactory {
	sharedPortConflictResolver := createPortConflictResolver()

	return &listenerConfiguratorFactory{
		unsupportedProtocol: &listenerConfigurator{
			validators: []listenerValidator{
				func(listener v1.Listener) ([]conditions.Condition, bool) {
					valErr := field.NotSupported(
						field.NewPath("protocol"),
						listener.Protocol,
						[]string{string(v1.HTTPProtocolType), string(v1.HTTPSProtocolType), string(v1.TLSProtocolType)},
					)
					return staticConds.NewListenerUnsupportedProtocol(valErr.Error()), false /* not attachable */
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
			},
			externalReferenceResolvers: []listenerExternalReferenceResolver{
				createExternalReferencesForTLSSecretsResolver(gw.Namespace, secretResolver, refGrantResolver),
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
			},
			externalReferenceResolvers: []listenerExternalReferenceResolver{},
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
}

func (c *listenerConfigurator) configure(listener v1.Listener) *Listener {
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
			msg := fmt.Sprintf("invalid label selector: %s", err.Error())
			conds = append(conds, staticConds.NewListenerUnsupportedValue(msg)...)
			valid = false
		}
	}

	supportedKinds := getListenerSupportedKinds(listener)

	l := &Listener{
		Name:                      string(listener.Name),
		Source:                    listener,
		Conditions:                conds,
		AllowedRouteLabelSelector: allowedRouteSelector,
		Routes:                    make(map[RouteKey]*L7Route),
		L4Routes:                  make(map[L4RouteKey]*L4Route),
		Valid:                     valid,
		Attachable:                attachable,
		SupportedKinds:            supportedKinds,
	}

	if !l.Valid {
		return l
	}

	// resolvers might add different conditions to the listener, so we run them all.

	for _, resolver := range c.conflictResolvers {
		resolver(l)
	}

	for _, resolver := range c.externalReferenceResolvers {
		resolver(l)
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
		return staticConds.NewListenerUnsupportedValue(valErr.Error()), false
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

	var validKinds []v1.RouteGroupKind

	switch listener.Protocol {
	case v1.HTTPProtocolType, v1.HTTPSProtocolType:
		validKinds = []v1.RouteGroupKind{
			{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
			{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
		}
	case v1.TLSProtocolType:
		validKinds = []v1.RouteGroupKind{
			{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
		}
	}

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
		supportedKinds = make([]v1.RouteGroupKind, 0, len(listener.AllowedRoutes.Kinds))
		for _, kind := range listener.AllowedRoutes.Kinds {
			if !validProtocolRouteKind(kind) {
				group := v1.GroupName
				if kind.Group != nil {
					group = string(*kind.Group)
				}
				msg := fmt.Sprintf("Unsupported route kind for protocol %s \"%s/%s\"", listener.Protocol, group, kind.Kind)
				conds = append(conds, staticConds.NewListenerInvalidRouteKinds(msg)...)
				continue
			}
			supportedKinds = append(supportedKinds, kind)
		}
		return conds, supportedKinds
	}

	return conds, validKinds
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
		return staticConds.NewListenerUnsupportedValue(msg), false
	}

	return nil, true
}

func createHTTPListenerValidator(protectedPorts ProtectedPorts) listenerValidator {
	return func(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
		if err := validateListenerPort(listener.Port, protectedPorts); err != nil {
			path := field.NewPath("port")
			valErr := field.Invalid(path, listener.Port, err.Error())
			conds = append(conds, staticConds.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if listener.TLS != nil {
			path := field.NewPath("tls")
			valErr := field.Forbidden(path, "tls is not supported for HTTP listener")
			conds = append(conds, staticConds.NewListenerUnsupportedValue(valErr.Error())...)
		}

		return conds, true
	}
}

func validateListenerPort(port v1.PortNumber, protectedPorts ProtectedPorts) error {
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1-65535")
	}

	if portName, ok := protectedPorts[int32(port)]; ok {
		return fmt.Errorf("port is already in use as %v", portName)
	}

	return nil
}

func validateTLSFieldOnTLSListener(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
	tlspath := field.NewPath("TLS")
	if listener.TLS == nil {
		valErr := field.Required(tlspath, "tls must be defined for TLS listener")
		return staticConds.NewListenerUnsupportedValue(valErr.Error()), false
	}
	if listener.TLS.Mode == nil || *listener.TLS.Mode != v1.TLSModePassthrough {
		valErr := field.Required(tlspath.Child("Mode"), "Mode must be passthrough for TLS listener")
		return staticConds.NewListenerUnsupportedValue(valErr.Error()), false
	}
	return nil, true
}

func createHTTPSListenerValidator(protectedPorts ProtectedPorts) listenerValidator {
	return func(listener v1.Listener) (conds []conditions.Condition, attachable bool) {
		if err := validateListenerPort(listener.Port, protectedPorts); err != nil {
			path := field.NewPath("port")
			valErr := field.Invalid(path, listener.Port, err.Error())
			conds = append(conds, staticConds.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if listener.TLS == nil {
			valErr := field.Required(field.NewPath("TLS"), "tls must be defined for HTTPS listener")
			conds = append(conds, staticConds.NewListenerUnsupportedValue(valErr.Error())...)
			return conds, true
		}

		tlsPath := field.NewPath("tls")

		if *listener.TLS.Mode != v1.TLSModeTerminate {
			valErr := field.NotSupported(
				tlsPath.Child("mode"),
				*listener.TLS.Mode,
				[]string{string(v1.TLSModeTerminate)},
			)
			conds = append(conds, staticConds.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if len(listener.TLS.Options) > 0 {
			path := tlsPath.Child("options")
			valErr := field.Forbidden(path, "options are not supported")
			conds = append(conds, staticConds.NewListenerUnsupportedValue(valErr.Error())...)
		}

		if len(listener.TLS.CertificateRefs) == 0 {
			msg := "certificateRefs must be defined for TLS mode terminate"
			valErr := field.Required(tlsPath.Child("certificateRefs"), msg)
			conds = append(conds, staticConds.NewListenerInvalidCertificateRef(valErr.Error())...)
			return conds, true
		}

		certRef := listener.TLS.CertificateRefs[0]

		certRefPath := tlsPath.Child("certificateRefs").Index(0)

		if certRef.Kind != nil && *certRef.Kind != "Secret" {
			path := certRefPath.Child("kind")
			valErr := field.NotSupported(path, *certRef.Kind, []string{"Secret"})
			conds = append(conds, staticConds.NewListenerInvalidCertificateRef(valErr.Error())...)
		}

		// for Kind Secret, certRef.Group must be nil or empty
		if certRef.Group != nil && *certRef.Group != "" {
			path := certRefPath.Child("group")
			valErr := field.NotSupported(path, *certRef.Group, []string{""})
			conds = append(conds, staticConds.NewListenerInvalidCertificateRef(valErr.Error())...)
		}

		if l := len(listener.TLS.CertificateRefs); l > 1 {
			path := tlsPath.Child("certificateRefs")
			valErr := field.TooMany(path, l, 1)
			conds = append(conds, staticConds.NewListenerUnsupportedValue(valErr.Error())...)
		}

		return conds, true
	}
}

func createPortConflictResolver() listenerConflictResolver {
	const (
		secureProtocolGroup   int = 0
		insecureProtocolGroup int = 1
	)
	protocolGroups := map[v1.ProtocolType]int{
		v1.TLSProtocolType:   secureProtocolGroup,
		v1.HTTPProtocolType:  insecureProtocolGroup,
		v1.HTTPSProtocolType: secureProtocolGroup,
	}
	conflictedPorts := make(map[v1.PortNumber]bool)
	portProtocolOwner := make(map[v1.PortNumber]int)
	listenersByPort := make(map[v1.PortNumber][]*Listener)

	format := "Multiple listeners for the same port %d specify incompatible protocols; " +
		"ensure only one protocol per port"

	formatHostname := "HTTPS and TLS listeners for the same port %d specify overlapping hostnames; " +
		"ensure no overlapping hostnames for HTTPS and TLS listeners for the same port"

	return func(l *Listener) {
		port := l.Source.Port

		// if port is in map of conflictedPorts then we only need to set the current listener to invalid
		if conflictedPorts[port] {
			l.Valid = false

			conflictedConds := staticConds.NewListenerProtocolConflict(fmt.Sprintf(format, port))
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

		// if protocol group owner doesn't match the listener's protocol group we mark the port as conflicted,
		// and invalidate all listeners we've seen for this port.
		if protocolGroup != protocolGroups[l.Source.Protocol] {
			conflictedPorts[port] = true
			for _, listener := range listenersByPort[port] {
				listener.Valid = false
				conflictedConds := staticConds.NewListenerProtocolConflict(fmt.Sprintf(format, port))
				listener.Conditions = append(listener.Conditions, conflictedConds...)
			}
			l.Valid = false
			conflictedConds := staticConds.NewListenerProtocolConflict(fmt.Sprintf(format, port))
			l.Conditions = append(l.Conditions, conflictedConds...)
		} else {
			foundConflict := false
			for _, listener := range listenersByPort[port] {
				if listener.Source.Protocol != l.Source.Protocol &&
					haveOverlap(l.Source.Hostname, listener.Source.Hostname) {
					listener.Valid = false
					conflictedConds := staticConds.NewListenerHostnameConflict(fmt.Sprintf(formatHostname, port))
					listener.Conditions = append(listener.Conditions, conflictedConds...)
					foundConflict = true
				}
			}

			if foundConflict {
				l.Valid = false
				conflictedConds := staticConds.NewListenerHostnameConflict(fmt.Sprintf(formatHostname, port))
				l.Conditions = append(l.Conditions, conflictedConds...)
			}
		}

		listenersByPort[port] = append(listenersByPort[port], l)
	}
}

func createExternalReferencesForTLSSecretsResolver(
	gwNs string,
	secretResolver *secretResolver,
	refGrantResolver *referenceGrantResolver,
) listenerExternalReferenceResolver {
	return func(l *Listener) {
		certRef := l.Source.TLS.CertificateRefs[0]

		certRefNs := gwNs
		if certRef.Namespace != nil {
			certRefNs = string(*certRef.Namespace)
		}

		certRefNsName := types.NamespacedName{
			Namespace: certRefNs,
			Name:      string(certRef.Name),
		}

		if certRefNs != gwNs {
			if !refGrantResolver.refAllowed(toSecret(certRefNsName), fromGateway(gwNs)) {
				msg := fmt.Sprintf("Certificate ref to secret %s not permitted by any ReferenceGrant", certRefNsName)

				l.Conditions = append(l.Conditions, staticConds.NewListenerRefNotPermitted(msg)...)
				l.Valid = false
				return
			}
		}

		if err := secretResolver.resolve(certRefNsName); err != nil {
			path := field.NewPath("tls", "certificateRefs").Index(0)
			// field.NotFound could be better, but it doesn't allow us to set the error message.
			valErr := field.Invalid(path, certRefNsName, err.Error())

			l.Conditions = append(l.Conditions, staticConds.NewListenerInvalidCertificateRef(valErr.Error())...)
			l.Valid = false
		} else {
			l.ResolvedSecret = &certRefNsName
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
			// Remove the "*." from h1
			h1 = h1[2:]
			// Check if h2 ends with h1
			return strings.HasSuffix(h2, h1)
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
