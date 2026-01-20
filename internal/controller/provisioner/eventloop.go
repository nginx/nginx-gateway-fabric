package provisioner

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	k8spredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller/predicate"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
	ngftypes "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

func newEventLoop(
	ctx context.Context,
	mgr manager.Manager,
	handler *eventHandler,
	logger logr.Logger,
	selector metav1.LabelSelector,
	ngfNamespace string,
	dockerSecrets []string,
	agentTLSSecret string,
	dataplaneKeySecret string,
	usageConfig *config.UsageReportConfig,
	isOpenshift bool,
	bigIPIngressLink bool,
) (*events.EventLoop, error) {
	nginxResourceLabelPredicate := predicate.NginxLabelPredicate(selector)

	secretsToWatch := make([]string, 0, len(dockerSecrets)+5)
	secretsToWatch = append(secretsToWatch, agentTLSSecret)
	secretsToWatch = append(secretsToWatch, dockerSecrets...)

	if dataplaneKeySecret != "" {
		secretsToWatch = append(secretsToWatch, dataplaneKeySecret)
	}

	if usageConfig != nil {
		if usageConfig.SecretName != "" {
			secretsToWatch = append(secretsToWatch, usageConfig.SecretName)
		}
		if usageConfig.CASecretName != "" {
			secretsToWatch = append(secretsToWatch, usageConfig.CASecretName)
		}
		if usageConfig.ClientSSLSecretName != "" {
			secretsToWatch = append(secretsToWatch, usageConfig.ClientSSLSecretName)
		}
	}

	type ctlrCfg struct {
		objectType ngftypes.ObjectType
		options    []controller.Option
	}

	controllerRegCfgs := []ctlrCfg{
		{
			objectType: &gatewayv1.Gateway{},
		},
		{
			objectType: &appsv1.Deployment{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						k8spredicate.GenerationChangedPredicate{},
						nginxResourceLabelPredicate,
						predicate.RestartDeploymentAnnotationPredicate{},
					),
				),
			},
		},
		{
			objectType: &appsv1.DaemonSet{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						k8spredicate.GenerationChangedPredicate{},
						nginxResourceLabelPredicate,
						predicate.RestartDeploymentAnnotationPredicate{},
					),
				),
			},
		},
		{
			objectType: &corev1.Service{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						nginxResourceLabelPredicate,
					),
				),
			},
		},
		{
			objectType: &corev1.ServiceAccount{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						k8spredicate.GenerationChangedPredicate{},
						nginxResourceLabelPredicate,
					),
				),
			},
		},
		{
			objectType: &corev1.ConfigMap{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						k8spredicate.GenerationChangedPredicate{},
						nginxResourceLabelPredicate,
					),
				),
			},
		},
		{
			objectType: &corev1.Secret{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						k8spredicate.ResourceVersionChangedPredicate{},
						k8spredicate.Or(
							nginxResourceLabelPredicate,
							predicate.SecretNamePredicate{Namespace: ngfNamespace, SecretNames: secretsToWatch},
						),
					),
				),
			},
		},
		{
			objectType: &autoscalingv2.HorizontalPodAutoscaler{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						nginxResourceLabelPredicate,
					),
				),
			},
		},
	}

	if isOpenshift {
		controllerRegCfgs = append(controllerRegCfgs,
			ctlrCfg{
				objectType: &rbacv1.Role{},
				options: []controller.Option{
					controller.WithK8sPredicate(
						k8spredicate.And(
							k8spredicate.GenerationChangedPredicate{},
							nginxResourceLabelPredicate,
						),
					),
				},
			},
			ctlrCfg{
				objectType: &rbacv1.RoleBinding{},
				options: []controller.Option{
					controller.WithK8sPredicate(
						k8spredicate.And(
							k8spredicate.GenerationChangedPredicate{},
							nginxResourceLabelPredicate,
						),
					),
				},
			},
		)
	}

	eventCh := make(chan any)

	// Register IngressLink controller separately since it uses unstructured type
	// which isn't in the scheme and requires special handling
	if bigIPIngressLink {
		ingressLinkObj := &unstructured.Unstructured{}
		ingressLinkObj.SetGroupVersionKind(ingressLinkGVK)
		if err := controller.Register(
			ctx,
			ingressLinkObj,
			"provisioner-IngressLink",
			mgr,
			eventCh,
			controller.WithK8sPredicate(predicate.IngressLinkStatusChangedPredicate{}),
		); err != nil {
			return nil, fmt.Errorf("cannot register controller for IngressLink: %w", err)
		}
	}
	for _, regCfg := range controllerRegCfgs {
		gvk, err := apiutil.GVKForObject(regCfg.objectType, mgr.GetScheme())
		if err != nil {
			panic(fmt.Sprintf("could not extract GVK for object: %T", regCfg.objectType))
		}

		if err := controller.Register(
			ctx,
			regCfg.objectType,
			fmt.Sprintf("provisioner-%s", gvk.Kind),
			mgr,
			eventCh,
			regCfg.options...,
		); err != nil {
			return nil, fmt.Errorf("cannot register controller for %T: %w", regCfg.objectType, err)
		}
	}

	objectList := []client.ObjectList{
		// GatewayList MUST be first in this list to ensure that we see it before attempting
		// to provision or deprovision any nginx resources.
		&gatewayv1.GatewayList{},
		&appsv1.DeploymentList{},
		&appsv1.DaemonSetList{},
		&autoscalingv2.HorizontalPodAutoscalerList{},
		&corev1.ServiceList{},
		&corev1.ServiceAccountList{},
		&corev1.ConfigMapList{},
		&corev1.SecretList{},
	}

	if isOpenshift {
		objectList = append(objectList,
			&rbacv1.RoleList{},
			&rbacv1.RoleBindingList{},
		)
	}

	firstBatchPreparer := events.NewFirstEventBatchPreparerImpl(
		mgr.GetCache(),
		[]client.Object{},
		objectList,
	)

	eventLoop := events.NewEventLoop(
		eventCh,
		logger.WithName("eventLoop"),
		handler,
		firstBatchPreparer,
	)

	return eventLoop, nil
}
