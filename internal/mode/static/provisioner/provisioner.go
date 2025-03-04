package provisioner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/status"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Provisioner

// Provisioner is an interface for triggering NGINX resources to be created/updated/deleted.
type Provisioner interface {
	RegisterGateway(ctx context.Context, gateway *graph.Gateway, resourceName string) error
}

// Config is the configuration for the Provisioner.
type Config struct {
	DeploymentStore  *agent.DeploymentStore
	StatusQueue      *status.Queue
	Logger           logr.Logger
	GatewayPodConfig config.GatewayPodConfig
	EventRecorder    record.EventRecorder
	GCName           string
	Plus             bool
}

// NginxProvisioner handles provisioning nginx kubernetes resources.
type NginxProvisioner struct {
	store     *store
	k8sClient client.Client
	// resourcesToDeleteOnStartup contains a list of Gateway names that no longer exist
	// but have nginx resources tied to them that need to be deleted.
	resourcesToDeleteOnStartup []types.NamespacedName
	baseLabelSelector          metav1.LabelSelector
	cfg                        Config
	leader                     bool

	lock sync.RWMutex
}

// NewNginxProvisioner returns a new instance of a Provisioner that will deploy nginx resources.
func NewNginxProvisioner(
	ctx context.Context,
	mgr manager.Manager,
	cfg Config,
) (*NginxProvisioner, *events.EventLoop, error) {
	store := newStore()

	selector := metav1.LabelSelector{
		MatchLabels: map[string]string{
			controller.AppInstanceLabel: cfg.GatewayPodConfig.InstanceName,
			controller.AppManagedByLabel: controller.CreateNginxResourceName(
				cfg.GatewayPodConfig.InstanceName,
				cfg.GCName,
			),
		},
	}

	provisioner := &NginxProvisioner{
		k8sClient:                  mgr.GetClient(),
		store:                      store,
		baseLabelSelector:          selector,
		resourcesToDeleteOnStartup: []types.NamespacedName{},
		cfg:                        cfg,
	}

	handler, err := newEventHandler(store, provisioner, selector, cfg.GCName)
	if err != nil {
		return nil, nil, fmt.Errorf("error initializing eventHandler: %w", err)
	}

	eventLoop, err := newEventLoop(ctx, mgr, handler, cfg.Logger, selector)
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
		if err := p.deprovisionNginx(ctx, gatewayNSName); err != nil {
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

//nolint:gocyclo // will refactor at some point
func (p *NginxProvisioner) provisionNginx(
	ctx context.Context,
	resourceName string,
	gateway *gatewayv1.Gateway,
	nProxyCfg *graph.EffectiveNginxProxy,
) error {
	if !p.isLeader() {
		return nil
	}

	objects := p.buildNginxResourceObjects(resourceName, gateway, nProxyCfg)

	p.cfg.Logger.Info(
		"Creating/Updating nginx resources",
		"namespace", gateway.GetNamespace(),
		"name", resourceName,
	)

	var agentConfigMapUpdated, deploymentCreated bool
	var deploymentObj *appsv1.Deployment
	for _, obj := range objects {
		createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		var res controllerutil.OperationResult
		if err := wait.PollUntilContextCancel(
			createCtx,
			500*time.Millisecond,
			true, /* poll immediately */
			func(ctx context.Context) (bool, error) {
				var upsertErr error
				res, upsertErr = controllerutil.CreateOrUpdate(ctx, p.k8sClient, obj, objectSpecSetter(obj))
				if upsertErr != nil {
					if !apierrors.IsAlreadyExists(upsertErr) && !apierrors.IsConflict(upsertErr) {
						return false, upsertErr
					}
					if apierrors.IsConflict(upsertErr) {
						return false, nil
					}
				}
				return true, nil
			},
		); err != nil {
			p.cfg.EventRecorder.Eventf(
				obj,
				corev1.EventTypeWarning,
				"CreateOrUpdateFailed",
				"Failed to create or update nginx resource: %s",
				err.Error(),
			)
			cancel()
			return err
		}
		cancel()

		if res != controllerutil.OperationResultCreated && res != controllerutil.OperationResultUpdated {
			continue
		}

		switch o := obj.(type) {
		case *appsv1.Deployment:
			deploymentObj = o
			if res == controllerutil.OperationResultCreated {
				deploymentCreated = true
			}
		case *corev1.ConfigMap:
			if res == controllerutil.OperationResultUpdated &&
				strings.Contains(obj.GetName(), nginxAgentConfigMapNameSuffix) {
				agentConfigMapUpdated = true
			}
		}

		result := cases.Title(language.English, cases.Compact).String(string(res))
		p.cfg.Logger.V(1).Info(
			fmt.Sprintf("%s nginx %s", result, obj.GetObjectKind().GroupVersionKind().Kind),
			"namespace", gateway.GetNamespace(),
			"name", resourceName,
		)
	}

	// if agent configmap was updated, then we'll need to restart the deployment
	if agentConfigMapUpdated && !deploymentCreated {
		updateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		p.cfg.Logger.V(1).Info(
			"Restarting nginx deployment after agent configmap update",
			"name", deploymentObj.GetName(),
			"namespace", deploymentObj.GetNamespace(),
		)

		if deploymentObj.Spec.Template.Annotations == nil {
			deploymentObj.Annotations = make(map[string]string)
		}
		deploymentObj.Spec.Template.Annotations[controller.RestartedAnnotation] = time.Now().Format(time.RFC3339)

		if err := p.k8sClient.Update(updateCtx, deploymentObj); err != nil && !apierrors.IsConflict(err) {
			p.cfg.EventRecorder.Eventf(
				deploymentObj,
				corev1.EventTypeWarning,
				"RestartFailed",
				"Failed to restart nginx deployment after agent config update: %s",
				err.Error(),
			)
			return err
		}
	}

	return nil
}

func (p *NginxProvisioner) reprovisionNginx(
	ctx context.Context,
	resourceName string,
	gateway *gatewayv1.Gateway,
	nProxyCfg *graph.EffectiveNginxProxy,
) error {
	if !p.isLeader() {
		return nil
	}
	objects := p.buildNginxResourceObjects(resourceName, gateway, nProxyCfg)

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
				corev1.EventTypeWarning,
				"CreateFailed",
				"Failed to create nginx resource: %s",
				err.Error(),
			)
			return err
		}
	}

	return nil
}

func (p *NginxProvisioner) deprovisionNginx(ctx context.Context, gatewayNSName types.NamespacedName) error {
	if !p.isLeader() {
		return nil
	}

	p.cfg.Logger.Info(
		"Removing nginx resources for Gateway",
		"name", gatewayNSName.Name,
		"namespace", gatewayNSName.Namespace,
	)

	deploymentNSName := types.NamespacedName{
		Name:      controller.CreateNginxResourceName(gatewayNSName.Name, p.cfg.GCName),
		Namespace: gatewayNSName.Namespace,
	}

	objects := p.buildNginxResourceObjectsForDeletion(deploymentNSName)

	createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, obj := range objects {
		if err := p.k8sClient.Delete(createCtx, obj); err != nil && !apierrors.IsNotFound(err) {
			p.cfg.EventRecorder.Eventf(
				obj,
				corev1.EventTypeWarning,
				"DeleteFailed",
				"Failed to delete nginx resource: %s",
				err.Error(),
			)
			return err
		}
	}

	p.store.deleteResourcesForGateway(gatewayNSName)
	p.cfg.DeploymentStore.Remove(deploymentNSName)

	return nil
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
	gatewayNSName := client.ObjectKeyFromObject(gateway.Source)
	if updated := p.store.registerResourceInGatewayConfig(gatewayNSName, gateway); !updated {
		return nil
	}

	if gateway.Valid {
		if err := p.provisionNginx(ctx, resourceName, gateway.Source, gateway.EffectiveNginxProxy); err != nil {
			return fmt.Errorf("error provisioning nginx resources: %w", err)
		}
	} else {
		if err := p.deprovisionNginx(ctx, gatewayNSName); err != nil {
			return fmt.Errorf("error deprovisioning nginx resources: %w", err)
		}
	}

	return nil
}
