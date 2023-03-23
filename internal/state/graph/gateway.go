package graph

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	nkgsort "github.com/nginxinc/nginx-kubernetes-gateway/internal/sort"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/conditions"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/state/secrets"
)

// Gateway represents the winning Gateway resource.
type Gateway struct {
	// Source is the corresponding Gateway resource.
	Source *v1beta1.Gateway
	// Listeners include the listeners of the Gateway.
	Listeners map[string]*Listener
}

// Listener represents a Listener of the Gateway resource.
// FIXME(pleshakov) For now, we only support HTTP and HTTPS listeners.
type Listener struct {
	// Source holds the source of the Listener from the Gateway resource.
	Source v1beta1.Listener
	// Routes holds the routes attached to the Listener.
	// Only valid routes are attached.
	Routes map[types.NamespacedName]*Route
	// AcceptedHostnames is an intersection between the hostnames supported by the Listener and the hostnames
	// from the attached routes.
	AcceptedHostnames map[string]struct{}
	// SecretPath is the path to the secret on disk.
	SecretPath string
	// Conditions holds the conditions of the Listener.
	Conditions []conditions.Condition
	// Valid shows whether the Listener is valid.
	// A Listener is considered valid if NKG can generate valid NGINX configuration for it.
	Valid bool
}

// processedGateways holds the resources that belong to NKG.
type processedGateways struct {
	Winner  *v1beta1.Gateway
	Ignored map[types.NamespacedName]*v1beta1.Gateway
}

// GetAllNsNames returns all the NamespacedNames of the Gateway resources that belong to NKG
func (gws processedGateways) GetAllNsNames() []types.NamespacedName {
	winnerCnt := 0
	if gws.Winner != nil {
		winnerCnt = 1
	}

	length := winnerCnt + len(gws.Ignored)
	if length == 0 {
		return nil
	}

	allNsNames := make([]types.NamespacedName, 0, length)

	if gws.Winner != nil {
		allNsNames = append(allNsNames, client.ObjectKeyFromObject(gws.Winner))
	}
	for nsName := range gws.Ignored {
		allNsNames = append(allNsNames, nsName)
	}

	return allNsNames
}

// processGateways determines which Gateway resource belong to NKG (determined by the Gateway GatewayClassName field).
func processGateways(
	gws map[types.NamespacedName]*v1beta1.Gateway,
	gcName string,
) processedGateways {
	referencedGws := make([]*v1beta1.Gateway, 0, len(gws))

	for _, gw := range gws {
		if string(gw.Spec.GatewayClassName) != gcName {
			continue
		}

		referencedGws = append(referencedGws, gw)
	}

	if len(referencedGws) == 0 {
		return processedGateways{}
	}

	sort.Slice(referencedGws, func(i, j int) bool {
		return nkgsort.LessObjectMeta(&referencedGws[i].ObjectMeta, &referencedGws[j].ObjectMeta)
	})

	ignoredGws := make(map[types.NamespacedName]*v1beta1.Gateway)

	for _, gw := range referencedGws[1:] {
		ignoredGws[client.ObjectKeyFromObject(gw)] = gw
	}

	return processedGateways{
		Winner:  referencedGws[0],
		Ignored: ignoredGws,
	}
}

func buildGateway(gw *v1beta1.Gateway, secretMemoryMgr secrets.SecretDiskMemoryManager) *Gateway {
	if gw == nil {
		return nil
	}

	return &Gateway{
		Source:    gw,
		Listeners: buildListeners(gw, secretMemoryMgr),
	}
}

func buildListeners(
	gw *v1beta1.Gateway,
	secretMemoryMgr secrets.SecretDiskMemoryManager,
) map[string]*Listener {
	listeners := make(map[string]*Listener)

	listenerFactory := newListenerConfiguratorFactory(gw, secretMemoryMgr)

	for _, gl := range gw.Spec.Listeners {
		configurator := listenerFactory.getConfiguratorForListener(gl)
		listeners[string(gl.Name)] = configurator.configure(gl)
	}

	return listeners
}

type listenerConfigurator interface {
	configure(listener v1beta1.Listener) *Listener
}

type listenerConfiguratorFactory struct {
	https *httpListenerConfigurator
	http  *httpListenerConfigurator
}

func (f *listenerConfiguratorFactory) getConfiguratorForListener(l v1beta1.Listener) listenerConfigurator {
	switch l.Protocol {
	case v1beta1.HTTPProtocolType:
		return f.http
	case v1beta1.HTTPSProtocolType:
		return f.https
	default:
		return newInvalidProtocolListenerConfigurator()
	}
}

func newListenerConfiguratorFactory(
	gw *v1beta1.Gateway,
	secretMemoryMgr secrets.SecretDiskMemoryManager,
) *listenerConfiguratorFactory {
	return &listenerConfiguratorFactory{
		https: newHTTPSListenerConfigurator(gw, secretMemoryMgr),
		http:  newHTTPListenerConfigurator(gw),
	}
}

type httpListenerConfigurator struct {
	gateway         *v1beta1.Gateway
	secretMemoryMgr secrets.SecretDiskMemoryManager
	usedHostnames   map[string]*Listener
	validate        func(gl v1beta1.Listener) []conditions.Condition
}

func newHTTPListenerConfigurator(gw *v1beta1.Gateway) *httpListenerConfigurator {
	return &httpListenerConfigurator{
		usedHostnames: make(map[string]*Listener),
		gateway:       gw,
		validate:      validateHTTPListener,
	}
}

func newHTTPSListenerConfigurator(
	gateway *v1beta1.Gateway,
	secretMemoryMgr secrets.SecretDiskMemoryManager,
) *httpListenerConfigurator {
	return &httpListenerConfigurator{
		gateway:         gateway,
		secretMemoryMgr: secretMemoryMgr,
		usedHostnames:   make(map[string]*Listener),
		validate: func(gl v1beta1.Listener) []conditions.Condition {
			return validateHTTPSListener(gl, gateway.Namespace)
		},
	}
}

func validateListener(
	gl v1beta1.Listener,
	gw *v1beta1.Gateway,
	validate func(gl v1beta1.Listener) []conditions.Condition,
) (conds []conditions.Condition, validHostname bool) {
	conds = validate(gl)

	if len(gw.Spec.Addresses) > 0 {
		path := field.NewPath("spec", "addresses")
		valErr := field.Forbidden(path, "addresses are not supported")
		conds = append(conds, conditions.NewListenerUnsupportedAddress(valErr.Error()))
	}

	validHostnameErr := validateListenerHostname(gl.Hostname)
	if validHostnameErr != nil {
		path := field.NewPath("hostname")
		valErr := field.Invalid(path, gl.Hostname, validHostnameErr.Error())
		conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error()))
	}

	return conds, validHostnameErr == nil
}

func (c *httpListenerConfigurator) ensureUniqueHostnamesAmongListeners(l *Listener) {
	h := getHostname(l.Source.Hostname)

	if holder, exist := c.usedHostnames[h]; exist {
		l.Valid = false

		holder.Valid = false   // all listeners for the same hostname become conflicted
		holder.SecretPath = "" // ensure secret path is unset for invalid listeners

		format := "Multiple listeners for the same port use the same hostname %q; " +
			"ensure only one listener uses that hostname"
		conflictedConds := conditions.NewListenerConflictedHostname(fmt.Sprintf(format, h))

		holder.Conditions = append(holder.Conditions, conflictedConds...)
		l.Conditions = append(l.Conditions, conflictedConds...)

		return
	}

	c.usedHostnames[h] = l
}

func (c *httpListenerConfigurator) loadSecretIntoListener(l *Listener) {
	if !l.Valid {
		return
	}

	nsname := types.NamespacedName{
		Namespace: c.gateway.Namespace,
		Name:      string(l.Source.TLS.CertificateRefs[0].Name),
	}

	var err error

	l.SecretPath, err = c.secretMemoryMgr.Request(nsname)
	if err != nil {
		path := field.NewPath("tls", "certificateRefs").Index(0)
		// field.NotFound could be better, but it doesn't allow us to set the error message.
		valErr := field.Invalid(path, nsname, err.Error())

		l.Conditions = append(l.Conditions, conditions.NewListenerInvalidCertificateRef(valErr.Error())...)
		l.Valid = false
	}
}

func (c *httpListenerConfigurator) configure(gl v1beta1.Listener) *Listener {
	// The functions called by configure() generate conditions for invalid fields of the listener.
	// Because the Gateway status includes a status field for each listener, the messages in those conditions
	// don't need to include the full path to the field (e.g. "spec.listeners[0].hostname"). They will include
	// a path starting from the field of a listener (e.g. "hostname", "tls.options").

	conds, validHostname := validateListener(gl, c.gateway, c.validate)

	l := &Listener{
		Source:            gl,
		Valid:             len(conds) == 0,
		Routes:            make(map[types.NamespacedName]*Route),
		AcceptedHostnames: make(map[string]struct{}),
		Conditions:        conds,
	}

	if validHostname {
		c.ensureUniqueHostnamesAmongListeners(l)
	}

	if gl.Protocol == v1beta1.HTTPSProtocolType {
		c.loadSecretIntoListener(l)
	}

	return l
}

type invalidProtocolListenerConfigurator struct{}

func newInvalidProtocolListenerConfigurator() *invalidProtocolListenerConfigurator {
	return &invalidProtocolListenerConfigurator{}
}

func (c *invalidProtocolListenerConfigurator) configure(gl v1beta1.Listener) *Listener {
	valErr := field.NotSupported(
		field.NewPath("protocol"),
		gl.Protocol,
		[]string{string(v1beta1.HTTPProtocolType), string(v1beta1.HTTPSProtocolType)},
	)

	return &Listener{
		Source:            gl,
		Valid:             false,
		Routes:            make(map[types.NamespacedName]*Route),
		AcceptedHostnames: make(map[string]struct{}),
		Conditions: []conditions.Condition{
			conditions.NewListenerUnsupportedProtocol(valErr.Error()),
		},
	}
}

func validateHTTPListener(listener v1beta1.Listener) []conditions.Condition {
	var conds []conditions.Condition

	if listener.Port != 80 {
		path := field.NewPath("port")
		valErr := field.NotSupported(path, listener.Port, []string{"80"})
		conds = append(conds, conditions.NewListenerPortUnavailable(valErr.Error()))
	}

	// The imported Webhook validation ensures the tls field is not set for an HTTP listener.
	// FIXME(pleshakov): Add a unit test for the imported Webhook validation code for this case.

	return conds
}

func validateHTTPSListener(listener v1beta1.Listener, gwNsName string) []conditions.Condition {
	var conds []conditions.Condition

	if listener.Port != 443 {
		path := field.NewPath("port")
		valErr := field.NotSupported(path, listener.Port, []string{"443"})
		conds = append(conds, conditions.NewListenerPortUnavailable(valErr.Error()))
	}

	// The imported Webhook validation ensures the tls field is not nil for an HTTPS listener.
	// FIXME(pleshakov): Add a unit test for the imported Webhook validation code for this case.

	tlsPath := field.NewPath("tls")

	if *listener.TLS.Mode != v1beta1.TLSModeTerminate {
		valErr := field.NotSupported(
			tlsPath.Child("mode"),
			*listener.TLS.Mode,
			[]string{string(v1beta1.TLSModeTerminate)},
		)
		conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error()))
	}

	if len(listener.TLS.Options) > 0 {
		path := tlsPath.Child("options")
		valErr := field.Forbidden(path, "options are not supported")
		conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error()))
	}

	// The imported Webhook validation ensures len(listener.TLS.Certificates) is not 0.
	// FIXME(pleshakov): Add a unit test for the imported Webhook validation code for this case.

	certRef := listener.TLS.CertificateRefs[0]

	certRefPath := tlsPath.Child("certificateRefs").Index(0)

	if certRef.Kind != nil && *certRef.Kind != "Secret" {
		path := certRefPath.Child("kind")
		valErr := field.NotSupported(path, *certRef.Kind, []string{"Secret"})
		conds = append(conds, conditions.NewListenerInvalidCertificateRef(valErr.Error())...)
	}

	// for Kind Secret, certRef.Group must be nil or empty
	if certRef.Group != nil && *certRef.Group != "" {
		path := certRefPath.Child("group")
		valErr := field.NotSupported(path, *certRef.Group, []string{""})
		conds = append(conds, conditions.NewListenerInvalidCertificateRef(valErr.Error())...)
	}

	// secret must be in the same namespace as the gateway
	if certRef.Namespace != nil && string(*certRef.Namespace) != gwNsName {
		const detail = "Referenced Secret must belong to the same namespace as the Gateway"
		path := certRefPath.Child("namespace")
		valErr := field.Invalid(path, certRef.Namespace, detail)
		conds = append(conds, conditions.NewListenerInvalidCertificateRef(valErr.Error())...)
	}

	if l := len(listener.TLS.CertificateRefs); l > 1 {
		path := tlsPath.Child("certificateRefs")
		valErr := field.TooMany(path, l, 1)
		conds = append(conds, conditions.NewListenerUnsupportedValue(valErr.Error()))
	}

	return conds
}

func validateListenerHostname(host *v1beta1.Hostname) error {
	if host == nil {
		return nil
	}

	h := string(*host)

	if h == "" {
		return nil
	}

	return validateHostname(h)
}
