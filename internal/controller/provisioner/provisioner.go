package provisioner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sEvents "k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/provisioner/openshift"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/telemetry"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

//go:generate go tool counterfeiter -generate

//counterfeiter:generate . Provisioner

// Provisioner is an interface for triggering NGINX resources to be created/updated/deleted.
type Provisioner interface {
	RegisterGateway(ctx context.Context, gateway *graph.Gateway, resourceName string) error
}

// Config is the configuration for the Provisioner.
type Config struct {
	DeploymentStore  agent.DeploymentStorer
	EventRecorder    k8sEvents.EventRecorder
	PlusUsageConfig  *config.UsageReportConfig
	StatusQueue      *status.Queue
	GatewayPodConfig *config.GatewayPodConfig
	AgentLabels      map[string]string
	Logger           logr.Logger
	GCName           string
	NGINXSCCName     string
	// GatewayCtlrName is the controller name string (from main config)
	GatewayCtlrName                string
	AgentTLSSecretName             string
	ServerTLSDomain                string
	NginxDockerSecretNames         []string
	NginxOneConsoleTelemetryConfig config.NginxOneConsoleTelemetryConfig
	Plus                           bool
	InferenceExtension             bool
	EndpointPickerDisableTLS       bool
	EndpointPickerTLSSkipVerify    bool
}

// NginxProvisioner handles provisioning nginx kubernetes resources.
type NginxProvisioner struct {
	k8sClient         client.Client
	store             *store
	baseLabelSelector metav1.LabelSelector
	// resourcesToDeleteOnStartup contains a list of Gateway names that no longer exist
	// but have nginx resources tied to them that need to be deleted.
	resourcesToDeleteOnStartup []types.NamespacedName
	cfg                        Config
	lock                       sync.RWMutex
	leader                     bool
	isOpenshift                bool
}

var apiChecker openshift.APIChecker = &openshift.APICheckerImpl{}

var labelCollectorFactory func(mgr manager.Manager, cfg Config) AgentLabelCollector = defaultLabelCollectorFactory

func defaultLabelCollectorFactory(mgr manager.Manager, cfg Config) AgentLabelCollector {
	return telemetry.NewLabelCollector(telemetry.LabelCollectorConfig{
		K8sClientReader: mgr.GetAPIReader(),
		Version:         cfg.GatewayPodConfig.Version,
		PodNSName: types.NamespacedName{
			Namespace: cfg.GatewayPodConfig.Namespace,
			Name:      cfg.GatewayPodConfig.Name,
		},
	})
}

type AgentLabelCollector interface {
	Collect(ctx context.Context) (map[string]string, error)
}

// NewNginxProvisioner returns a new instance of a Provisioner that will deploy nginx resources.
func NewNginxProvisioner(
	ctx context.Context,
	mgr manager.Manager,
	cfg Config,
) (*NginxProvisioner, *events.EventLoop, error) {
	var jwtSecretName, caSecretName, clientSSLSecretName string
	if cfg.Plus && cfg.PlusUsageConfig != nil {
		jwtSecretName = cfg.PlusUsageConfig.SecretName
		caSecretName = cfg.PlusUsageConfig.CASecretName
		clientSSLSecretName = cfg.PlusUsageConfig.ClientSSLSecretName
	}

	var dataplaneKeySecretName string
	if cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "" {
		dataplaneKeySecretName = cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName
	}

	store := newStore(
		cfg.NginxDockerSecretNames,
		cfg.AgentTLSSecretName,
		jwtSecretName,
		caSecretName,
		clientSSLSecretName,
		dataplaneKeySecretName,
	)

	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			controller.AppInstanceLabel: cfg.GatewayPodConfig.InstanceName,
			controller.AppManagedByLabel: controller.CreateNginxResourceName(
				cfg.GatewayPodConfig.InstanceName,
				cfg.GCName,
			),
		},
	}

	isOpenshift, err := apiChecker.IsOpenshift(mgr.GetConfig())
	if err != nil {
		cfg.Logger.Error(err, "could not determine if running in openshift, will not create Role/RoleBinding")
	}

	agentLabelCollector := labelCollectorFactory(mgr, cfg)
	agentLabels, err := agentLabelCollector.Collect(ctx)
	if err != nil {
		cfg.Logger.Error(err, "failed to collect agent labels")
	}
	cfg.AgentLabels = agentLabels
	if cfg.AgentLabels == nil {
		cfg.AgentLabels = make(map[string]string)
	}

	provisioner := &NginxProvisioner{
		k8sClient:                  mgr.GetClient(),
		store:                      store,
		baseLabelSelector:          selector,
		resourcesToDeleteOnStartup: []types.NamespacedName{},
		cfg:                        cfg,
		isOpenshift:                isOpenshift,
	}

	handler, err := newEventHandler(store, provisioner, mgr.GetClient(), selector, cfg.GCName)
	if err != nil {
		return nil, nil, fmt.Errorf("error initializing eventHandler: %w", err)
	}

	eventLoop, err := newEventLoop(
		ctx,
		mgr,
		handler,
		cfg.Logger,
		selector,
		cfg.GatewayPodConfig.Namespace,
		cfg.NginxDockerSecretNames,
		cfg.AgentTLSSecretName,
		dataplaneKeySecretName,
		cfg.PlusUsageConfig,
		isOpenshift,
	)
	if err != nil {
		return nil, nil, err
	}

	return provisioner, eventLoop, nil
}

// Enable is called when the Pod becomes leader and allows the provisioner to manage resources.
func (p *NginxProvisioner) Enable(ctx context.Context) {
	p.lock.Lock()
	p.leader = true
	p.lock.Unlock()

	p.lock.RLock()
	for _, gatewayNSName := range p.resourcesToDeleteOnStartup {
		if p.store.getGateway(gatewayNSName) != nil {
			continue
		}
		if err := p.deprovisionNginxForInvalidGateway(ctx, gatewayNSName); err != nil {
			p.cfg.Logger.Error(err, "error deprovisioning nginx resources on startup")
		}
	}
	p.lock.RUnlock()

	p.lock.Lock()
	p.resourcesToDeleteOnStartup = []types.NamespacedName{}
	p.lock.Unlock()
}

// isLeader returns whether or not this provisioner is the leader.
func (p *NginxProvisioner) isLeader() bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.leader
}

// setResourceToDelete is called when there are resources to delete, but this pod is not leader.
// Once it becomes leader, it will delete those resources.
func (p *NginxProvisioner) setResourceToDelete(gatewayNSName types.NamespacedName) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.resourcesToDeleteOnStartup = append(p.resourcesToDeleteOnStartup, gatewayNSName)
}

// patchServiceStatus updates the Service.status.loadBalancer.ingress with the provided IPs.
func (p *NginxProvisioner) patchServiceStatus(ctx context.Context, namespace, name string, ips []string) error {
	if len(ips) == 0 {
		return nil
	}

	svc := &corev1.Service{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := p.k8sClient.Get(ctx, key, svc); err != nil {
		return fmt.Errorf("failed to get Service for status patch: %w", err)
	}

	// Ensure this Service appears to belong to NGF by checking managed-by and instance labels.
	// If it doesn't, avoid modifying status of unrelated Services.
	managedBy := svc.Labels[controller.AppManagedByLabel]
	instance := svc.Labels[controller.AppInstanceLabel]
	expectedManagedBy := controller.CreateNginxResourceName(p.cfg.GatewayPodConfig.InstanceName, p.cfg.GCName)
	if instance != p.cfg.GatewayPodConfig.InstanceName || managedBy != expectedManagedBy {
		p.cfg.Logger.V(1).Info(
			"skipping status patch for Service that is not managed by NGF",
			"service", fmt.Sprintf("%s/%s", namespace, name),
		)
		return nil
	}

	// Build a map of existing ingress entries
	ingress := createUniqueIngressList(svc, ips)

	// If the desired IPs already match the existing IPs (order-sensitive), nothing to do.
	existingIPs := make([]string, 0, len(svc.Status.LoadBalancer.Ingress))
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		existingIPs = append(existingIPs, ingress.IP)
	}
	desiredIPs := make([]string, 0, len(ingress))
	for _, ingress := range ingress {
		desiredIPs = append(desiredIPs, ingress.IP)
	}
	if slices.Equal(existingIPs, desiredIPs) {
		return nil
	}

	// Patch the status subresource using MergeFrom to avoid clobbering concurrent updates.
	original := svc.DeepCopy()
	svc.Status.LoadBalancer.Ingress = ingress

	backoff := wait.Backoff{Steps: 5, Duration: 100 * time.Millisecond, Factor: 2.0, Jitter: 0.1}
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		if err := p.k8sClient.Status().Patch(ctx, svc, client.MergeFrom(original)); err != nil {
			p.cfg.Logger.V(1).Info(
				"Encountered error patching service status",
				"error", err,
				"namespace", svc.Namespace,
				"name", svc.Name,
				"kind", svc.GetObjectKind().GroupVersionKind().Kind,
			)

			if apierrors.IsConflict(err) {
				// Refresh original and svc and retry
				if getErr := p.k8sClient.Get(ctx, key, original); getErr != nil {
					if apierrors.IsNotFound(getErr) {
						return true, getErr
					}
					return false, nil
				}
				// apply desired ingress onto a fresh copy
				svc = original.DeepCopy()
				svc.Status.LoadBalancer.Ingress = ingress
				return false, nil
			}

			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to patch Service status: %w", err)
	}

	return nil
}

// createUniqueIngressList takes the existing Service and the desired list of IPs,
// and returns a list of LoadBalancerIngress.
func createUniqueIngressList(svc *corev1.Service, ips []string) []corev1.LoadBalancerIngress {
	existingByIP := make(map[string]corev1.LoadBalancerIngress, len(svc.Status.LoadBalancer.Ingress))
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		existingByIP[ingress.IP] = ingress
	}

	// Build unique ingress list preserving order and any existing fields.
	seen := make(map[string]struct{})
	ingress := make([]corev1.LoadBalancerIngress, 0, len(ips))
	for _, ip := range ips {
		if ip == "" || net.ParseIP(ip) == nil {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		if existing, ok := existingByIP[ip]; ok {
			ingress = append(ingress, existing)
		} else {
			ingress = append(ingress, corev1.LoadBalancerIngress{IP: ip})
		}
	}
	return ingress
}

// minimalObjectFactory is a map of constructors for creating minimal objects with only name and namespace set.
var minimalObjectFactory = map[reflect.Type]func(name, namespace string) client.Object{
	reflect.TypeOf(&appsv1.Deployment{}): func(name, namespace string) client.Object {
		return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&appsv1.DaemonSet{}): func(name, namespace string) client.Object {
		return &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&corev1.Service{}): func(name, namespace string) client.Object {
		return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&corev1.ServiceAccount{}): func(name, namespace string) client.Object {
		return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&corev1.ConfigMap{}): func(name, namespace string) client.Object {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&corev1.Secret{}): func(name, namespace string) client.Object {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&rbacv1.Role{}): func(name, namespace string) client.Object {
		return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&rbacv1.RoleBinding{}): func(name, namespace string) client.Object {
		return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&autoscalingv2.HorizontalPodAutoscaler{}): func(name, namespace string) client.Object {
		return &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
	reflect.TypeOf(&policyv1.PodDisruptionBudget{}): func(name, namespace string) client.Object {
		return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	},
}

// createMinimalClone creates a new object of the same type with only name and namespace set.
// This follows CreateOrUpdate's requirement that only name/namespace should be set on the input object.
func createMinimalClone(obj client.Object) client.Object {
	objType := reflect.TypeOf(obj)
	factory, exists := minimalObjectFactory[objType]
	if !exists {
		panic(fmt.Errorf("failed to create minimal clone: no factory mapping for object type %T", obj))
	}

	// A new object will be created by this factory function
	return factory(obj.GetName(), obj.GetNamespace())
}

func (p *NginxProvisioner) provisionNginx(
	ctx context.Context,
	resourceName string,
	gateway *gatewayv1.Gateway,
	objects []client.Object,
) error {
	if !p.isLeader() {
		return nil
	}

	objNames := make([]string, 0, len(objects))
	for _, obj := range objects {
		objNames = append(objNames, fmt.Sprintf("%s (%s)", obj.GetName(), reflect.TypeOf(obj).Elem().Name()))
	}

	p.cfg.Logger.Info(
		"Creating/Updating nginx resources",
		"namespace", gateway.GetNamespace(),
		"nginx resource name", resourceName,
		"resource names", objNames,
	)

	var state nginxProvisionState
	for _, obj := range objects {
		minimalObj, res, err := p.createOrUpdateNginxResource(ctx, resourceName, gateway, obj)
		if err != nil {
			return err
		}

		state.track(minimalObj, res)

		// If the Service is a LoadBalancer and the Gateway declares IP-type addresses,
		// patch the Service status with those IPs.
		if svc, ok := minimalObj.(*corev1.Service); ok {
			p.patchLoadBalancerServiceStatus(ctx, svc, gateway)
		}

		if res != controllerutil.OperationResultCreated && res != controllerutil.OperationResultUpdated {
			p.cfg.Logger.V(1).Info(
				"nginx resource already up to date with this result: "+string(res),
				"namespace", gateway.GetNamespace(),
				"name", fmt.Sprintf("%s (%s)", resourceName, reflect.TypeOf(minimalObj).Elem().Name()),
			)
			continue
		}

		result := cases.Title(language.English, cases.Compact).String(string(res))
		p.cfg.Logger.V(1).Info(
			fmt.Sprintf("%s nginx %s", result, reflect.TypeOf(minimalObj).Elem().Name()),
			"namespace", gateway.GetNamespace(),
			"name", resourceName,
		)
		p.store.registerResourceInGatewayConfig(client.ObjectKeyFromObject(gateway), minimalObj)
	}

	// if agent configmap was updated, then we'll need to restart the deployment/daemonset
	return p.restartNginxAfterConfigUpdate(ctx, gateway, state)
}

// nginxProvisionState tracks the resources observed while provisioning a Gateway's nginx objects,
// so that an agent ConfigMap update can trigger a restart of the corresponding Deployment/DaemonSet.
type nginxProvisionState struct {
	deploymentObj         *appsv1.Deployment
	daemonSetObj          *appsv1.DaemonSet
	agentConfigMapUpdated bool
	deploymentCreated     bool
}

// track records the result of creating or updating a single nginx resource.
func (s *nginxProvisionState) track(minimalObj client.Object, res controllerutil.OperationResult) {
	switch o := minimalObj.(type) {
	case *appsv1.Deployment:
		s.deploymentObj = o
		if res == controllerutil.OperationResultCreated {
			s.deploymentCreated = true
		}
	case *appsv1.DaemonSet:
		s.daemonSetObj = o
		if res == controllerutil.OperationResultCreated {
			s.deploymentCreated = true
		}
	case *corev1.ConfigMap:
		if res == controllerutil.OperationResultUpdated &&
			strings.HasSuffix(minimalObj.GetName(), nginxAgentConfigMapNameSuffix) {
			s.agentConfigMapUpdated = true
		}
	}
}

// createOrUpdateNginxResource creates or updates a single nginx resource, retrying on transient
// errors. It returns the minimal clone that was reconciled along with the operation result. On
// failure it records a warning event on the Gateway and returns the error.
func (p *NginxProvisioner) createOrUpdateNginxResource(
	ctx context.Context,
	resourceName string,
	gateway *gatewayv1.Gateway,
	obj client.Object,
) (client.Object, controllerutil.OperationResult, error) {
	// Create a minimal clone with only name and namespace for CreateOrUpdate
	// This follows the CreateOrUpdate documentation that says only name/namespace should be set
	minimalObj := createMinimalClone(obj)

	createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var res controllerutil.OperationResult
	var upsertErr error
	if err := wait.PollUntilContextCancel(
		createCtx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			// Use minimalObj for CreateOrUpdate but pass both to objectSpecSetter so we can transfer
			// the desired spec and other meta details of the object
			res, upsertErr = controllerutil.CreateOrUpdate(ctx, p.k8sClient, minimalObj, objectSpecSetter(minimalObj, obj))

			if upsertErr != nil {
				if apierrors.IsInvalid(upsertErr) { // log this error at the error level
					// spec.loadBalancerClass is immutable in Kubernetes; it cannot be changed after
					// a Service is created. We cannot delete and recreate the Service because that
					// would release and re-allocate the external IP, breaking DNS for users.
					// Return the error to stop retrying; the outer handler will log and surface it.
					if _, ok := obj.(*corev1.Service); ok && isLoadBalancerClassImmutabilityErr(upsertErr) {
						return false, upsertErr
					}
					p.cfg.Logger.Error(
						upsertErr,
						"Retrying CreateOrUpdate for nginx resource after error",
						"namespace", gateway.GetNamespace(),
						"name", fmt.Sprintf("%s (%s)", resourceName, reflect.TypeOf(obj).Elem().Name()),
					)
				} else {
					p.cfg.Logger.V(1).Info(
						"Retrying CreateOrUpdate for nginx resource after error",
						"namespace", gateway.GetNamespace(),
						"name", fmt.Sprintf("%s (%s)", resourceName, reflect.TypeOf(obj).Elem().Name()),
						"error", upsertErr.Error(),
					)
				}
				return false, nil
			}
			return true, nil
		},
	); err != nil {
		p.cfg.Logger.Error(
			err,
			"Failed to CreateOrUpdate nginx resource after retries",
			"namespace", gateway.GetNamespace(),
			"name", fmt.Sprintf("%s (%s)", resourceName, reflect.TypeOf(obj).Elem().Name()),
		)

		fullErr := errors.Join(err, upsertErr)
		p.cfg.EventRecorder.Eventf(
			obj,
			gateway,
			corev1.EventTypeWarning,
			"CreateOrUpdateFailed",
			"None",
			"Failed to create or update nginx resource: %s",
			fullErr.Error(),
		)
		return nil, res, fullErr
	}

	return minimalObj, res, nil
}

// patchLoadBalancerServiceStatus patches a LoadBalancer Service's status with the Gateway's
// IP-type addresses when the Service is managed by this controller. Patch failures are logged
// but not returned, since they should not block provisioning.
func (p *NginxProvisioner) patchLoadBalancerServiceStatus(
	ctx context.Context,
	svc *corev1.Service,
	gateway *gatewayv1.Gateway,
) {
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return
	}

	var ips []string
	for _, addr := range gateway.Spec.Addresses {
		if addr.Type != nil && *addr.Type == gatewayv1.IPAddressType {
			ips = append(ips, addr.Value)
		}
	}

	if svc.Spec.LoadBalancerClass != nil && *svc.Spec.LoadBalancerClass == p.cfg.GatewayCtlrName && len(ips) > 0 {
		if err := p.patchServiceStatus(ctx, svc.GetNamespace(), svc.GetName(), ips); err != nil {
			p.cfg.Logger.Error(
				err,
				"failed to patch Service status with gateway external IPs",
				"service", fmt.Sprintf("%s/%s", svc.GetNamespace(), svc.GetName()),
			)
		}
	}
}

// restartNginxAfterConfigUpdate restarts the provisioned Deployment/DaemonSet when the agent
// ConfigMap was updated without the workload itself being newly created, so that nginx picks up
// the new agent configuration.
func (p *NginxProvisioner) restartNginxAfterConfigUpdate(
	ctx context.Context,
	gateway *gatewayv1.Gateway,
	state nginxProvisionState,
) error {
	if !state.agentConfigMapUpdated || state.deploymentCreated {
		return nil
	}

	updateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var object client.Object
	if state.deploymentObj != nil {
		if state.deploymentObj.Spec.Template.Annotations == nil {
			state.deploymentObj.Spec.Template.Annotations = make(map[string]string)
		}
		state.deploymentObj.Spec.Template.Annotations[controller.RestartedAnnotation] = time.Now().Format(time.RFC3339)
		object = state.deploymentObj
	} else if state.daemonSetObj != nil {
		if state.daemonSetObj.Spec.Template.Annotations == nil {
			state.daemonSetObj.Spec.Template.Annotations = make(map[string]string)
		}
		state.daemonSetObj.Spec.Template.Annotations[controller.RestartedAnnotation] = time.Now().Format(time.RFC3339)
		object = state.daemonSetObj
	}

	if object == nil {
		return nil
	}

	p.cfg.Logger.V(1).Info(
		"Restarting nginx after agent configmap update",
		"name", object.GetName(),
		"namespace", object.GetNamespace(),
	)

	if err := p.k8sClient.Update(updateCtx, object); err != nil && !apierrors.IsConflict(err) {
		p.cfg.EventRecorder.Eventf(
			object,
			gateway,
			corev1.EventTypeWarning,
			"RestartFailed",
			"None",
			"Failed to restart nginx after agent config update: %s",
			err.Error(),
		)
		return err
	}

	return nil
}

func (p *NginxProvisioner) reprovisionNginx(
	ctx context.Context,
	resourceName string,
	gateway *gatewayv1.Gateway,
	nProxyCfg *graph.EffectiveNginxProxy,
	allListeners []*graph.Listener,
) error {
	if !p.isLeader() {
		return nil
	}
	if len(allListeners) == 0 {
		return nil
	}
	objects, err := p.buildNginxResourceObjects(resourceName, gateway, nProxyCfg, allListeners)
	if err != nil {
		p.cfg.Logger.Error(err, "error provisioning some nginx resources")
	}

	p.cfg.Logger.Info(
		"Re-creating nginx resources",
		"namespace", gateway.GetNamespace(),
		"name", resourceName,
	)

	createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, obj := range objects {
		if err := p.k8sClient.Create(createCtx, obj); err != nil && !apierrors.IsAlreadyExists(err) {
			p.cfg.EventRecorder.Eventf(
				obj,
				gateway,
				corev1.EventTypeWarning,
				"CreateFailed",
				"None",
				"Failed to create nginx resource: %s",
				err.Error(),
			)
			return err
		}
	}

	return nil
}

func (p *NginxProvisioner) deprovisionNginxForInvalidGateway(
	ctx context.Context,
	gatewayNSName types.NamespacedName,
) error {
	deploymentNSName := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(gatewayNSName.Name, p.cfg.GCName),
		Namespace: gatewayNSName.Namespace,
	}

	if p.isLeader() {
		p.cfg.Logger.Info(
			"Removing nginx resources for Gateway",
			"name", gatewayNSName.Name,
			"namespace", gatewayNSName.Namespace,
		)

		objects := p.buildResourcesForInvalidGatewayCleanup(deploymentNSName)

		deleteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		for _, obj := range objects {
			if err := p.k8sClient.Delete(deleteCtx, obj); err != nil && !apierrors.IsNotFound(err) {
				p.cfg.EventRecorder.Eventf(
					obj,
					&gatewayv1.Gateway{
						ObjectMeta: metav1.ObjectMeta{
							Name:      gatewayNSName.Name,
							Namespace: gatewayNSName.Namespace,
						},
					},
					corev1.EventTypeWarning,
					"DeleteFailed",
					"None",
					"Failed to delete nginx resource: %s",
					err.Error(),
				)
				return err
			}
		}
	}

	p.store.deleteResourcesForGateway(gatewayNSName)
	p.cfg.DeploymentStore.Remove(deploymentNSName)

	return nil
}

func (p *NginxProvisioner) deleteObject(ctx context.Context, obj client.Object) error {
	if !p.isLeader() {
		return nil
	}

	deleteCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := p.k8sClient.Delete(deleteCtx, obj); err != nil && !apierrors.IsNotFound(err) {
		p.cfg.EventRecorder.Eventf(
			obj,
			nil,
			corev1.EventTypeWarning,
			"DeleteFailed",
			"None",
			"Failed to delete nginx resource: %s",
			err.Error(),
		)
		return err
	}

	return nil
}

// isUserSecret determines if the provided secret name is a special user secret,
// for example an NGINX docker registry secret or NGINX Plus secret.
func (p *NginxProvisioner) isUserSecret(name string) bool {
	if name == p.cfg.AgentTLSSecretName {
		return true
	}

	if slices.Contains(p.cfg.NginxDockerSecretNames, name) {
		return true
	}

	if p.cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName == name {
		return true
	}

	if p.cfg.PlusUsageConfig != nil {
		return name == p.cfg.PlusUsageConfig.SecretName ||
			name == p.cfg.PlusUsageConfig.CASecretName ||
			name == p.cfg.PlusUsageConfig.ClientSSLSecretName
	}

	return false
}

// RegisterGateway is called by the main event handler when a Gateway API resource event occurs
// and the graph is built. The provisioner updates the Gateway config in the store and then:
// - If it's a valid Gateway, create or update nginx resources associated with the Gateway, if necessary.
// - If it's an invalid Gateway, delete the associated nginx resources.
func (p *NginxProvisioner) RegisterGateway(
	ctx context.Context,
	gateway *graph.Gateway,
	resourceName string,
) error {
	if !p.isLeader() {
		return nil
	}

	gatewayNSName := client.ObjectKeyFromObject(gateway.Source)
	if updated := p.store.registerResourceInGatewayConfig(gatewayNSName, gateway); !updated {
		return nil
	}

	if gateway.Valid && len(gateway.Listeners) > 0 {
		objects, err := p.buildNginxResourceObjects(
			resourceName,
			gateway.Source,
			gateway.EffectiveNginxProxy,
			gateway.Listeners,
		)
		if err != nil {
			p.cfg.Logger.Error(err, "error building some nginx resources")
		}

		// If NGINX deployment type switched between Deployment and DaemonSet, clean up the old one.
		// If HPA was disabled, remove it.
		nginxResources := p.store.getNginxResourcesForGateway(gatewayNSName)
		if nginxResources != nil {
			p.handleObjectDeletion(ctx, nginxResources)
		}

		if err := p.provisionNginx(ctx, resourceName, gateway.Source, objects); err != nil {
			return fmt.Errorf("error provisioning nginx resources: %w", err)
		}
	} else {
		if err := p.deprovisionNginxForInvalidGateway(ctx, gatewayNSName); err != nil {
			return fmt.Errorf("error deprovisioning nginx resources: %w", err)
		}
	}

	return nil
}

func (p *NginxProvisioner) handleObjectDeletion(ctx context.Context, nginxResources *NginxResources) {
	if needToDeleteDaemonSet(nginxResources) {
		if err := p.deleteObject(ctx, &appsv1.DaemonSet{ObjectMeta: nginxResources.DaemonSet}); err != nil {
			p.cfg.Logger.Error(err, "error deleting nginx resource")
		}
	} else if needToDeleteDeployment(nginxResources) {
		if err := p.deleteObject(ctx, &appsv1.Deployment{ObjectMeta: nginxResources.Deployment}); err != nil {
			p.cfg.Logger.Error(err, "error deleting nginx resource")
		}
	}

	if needToDeleteHPA(nginxResources) {
		if err := p.deleteObject(ctx, &autoscalingv2.HorizontalPodAutoscaler{ObjectMeta: nginxResources.HPA}); err != nil {
			p.cfg.Logger.Error(err, "error deleting nginx resource")
		}
	}

	if needToDeletePDB(nginxResources) {
		if err := p.deleteObject(ctx, &policyv1.PodDisruptionBudget{ObjectMeta: nginxResources.PDB}); err != nil {
			p.cfg.Logger.Error(err, "error deleting nginx resource")
		}
	}
}

func needToDeleteDeployment(cfg *NginxResources) bool {
	if cfg.Deployment.Name != "" {
		if cfg.Gateway != nil && cfg.Gateway.EffectiveNginxProxy != nil &&
			cfg.Gateway.EffectiveNginxProxy.Kubernetes != nil &&
			cfg.Gateway.EffectiveNginxProxy.Kubernetes.DaemonSet != nil {
			return true
		}
	}

	return false
}

func needToDeleteDaemonSet(cfg *NginxResources) bool {
	if cfg.DaemonSet.Name != "" && cfg.Gateway != nil {
		if cfg.Gateway.EffectiveNginxProxy != nil &&
			cfg.Gateway.EffectiveNginxProxy.Kubernetes != nil &&
			cfg.Gateway.EffectiveNginxProxy.Kubernetes.Deployment != nil {
			return true
		} else if cfg.Gateway.EffectiveNginxProxy == nil ||
			cfg.Gateway.EffectiveNginxProxy.Kubernetes == nil ||
			cfg.Gateway.EffectiveNginxProxy.Kubernetes.DaemonSet == nil {
			return true
		}
	}

	return false
}

// isLoadBalancerClassImmutabilityErr returns true when the error is a Kubernetes validation
// error for the spec.loadBalancerClass field, which is immutable once a Service is created.
func isLoadBalancerClassImmutabilityErr(err error) bool {
	var statusErr *apierrors.StatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	if statusErr.ErrStatus.Details == nil {
		return false
	}
	for _, cause := range statusErr.ErrStatus.Details.Causes {
		if cause.Field == "spec.loadBalancerClass" {
			return true
		}
	}
	return false
}

// needToDeletePDB returns true if a PDB was previously created for this Gateway
// but is no longer configured in the NginxProxy spec, and therefore should be deleted.
func needToDeletePDB(cfg *NginxResources) bool {
	if cfg.PDB.Name != "" && cfg.Gateway != nil {
		if cfg.Gateway.EffectiveNginxProxy != nil &&
			cfg.Gateway.EffectiveNginxProxy.Kubernetes != nil &&
			(cfg.Gateway.EffectiveNginxProxy.Kubernetes.Deployment == nil ||
				cfg.Gateway.EffectiveNginxProxy.Kubernetes.Deployment.PodDisruptionBudget == nil) {
			return true
		} else if cfg.Gateway.EffectiveNginxProxy == nil ||
			cfg.Gateway.EffectiveNginxProxy.Kubernetes == nil {
			return true
		}
	}

	return false
}

// needToDeleteHPA returns true if an HPA was previously created for this Gateway
// but is no longer configured in the NginxProxy spec, and therefore should be deleted.
func needToDeleteHPA(cfg *NginxResources) bool {
	if cfg.HPA.Name != "" && cfg.Gateway != nil {
		if cfg.Gateway.EffectiveNginxProxy != nil &&
			cfg.Gateway.EffectiveNginxProxy.Kubernetes != nil &&
			!isAutoscalingEnabled(cfg.Gateway.EffectiveNginxProxy.Kubernetes.Deployment) {
			return true
		} else if cfg.Gateway.EffectiveNginxProxy == nil ||
			cfg.Gateway.EffectiveNginxProxy.Kubernetes == nil {
			return true
		}
	}

	return false
}
