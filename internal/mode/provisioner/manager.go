package provisioner

import (
	"fmt"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	embeddedfiles "github.com/nginx/nginx-gateway-fabric"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/predicate"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/gatewayclass"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/status"
	ngftypes "github.com/nginx/nginx-gateway-fabric/internal/framework/types"
)

// Config is configuration for the provisioner mode.
type Config struct {
	Logger           logr.Logger
	GatewayClassName string
	GatewayCtlrName  string
}

// StartManager starts a Manager for the provisioner mode, which provisions
// a Deployment of NGF (static mode) for each Gateway of the provisioner GatewayClass.
//
// The provisioner mode is introduced to allow running Gateway API conformance tests for NGF, which expects
// an independent data plane instance being provisioned for each Gateway.
//
// The provisioner mode is not intended to be used in production (in the short term), as it lacks support for
// many important features. See https://github.com/nginx/nginx-gateway-fabric/issues/634 for more details.
func StartManager(cfg Config) error {
	scheme := runtime.NewScheme()
	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(apiext.AddToScheme(scheme))

	options := manager.Options{
		Scheme: scheme,
		Logger: cfg.Logger,
	}
	clusterCfg := ctlr.GetConfigOrDie()

	mgr, err := manager.New(clusterCfg, options)
	if err != nil {
		return fmt.Errorf("cannot build runtime manager: %w", err)
	}

	crdWithGVK := apiext.CustomResourceDefinition{}
	crdWithGVK.SetGroupVersionKind(
		schema.GroupVersionKind{Group: apiext.GroupName, Version: "v1", Kind: "CustomResourceDefinition"},
	)

	// Note: for any new object type or a change to the existing one,
	// make sure to also update firstBatchPreparer creation below
	controllerRegCfgs := []struct {
		objectType ngftypes.ObjectType
		options    []controller.Option
	}{
		{
			objectType: &gatewayv1.GatewayClass{},
			options: []controller.Option{
				controller.WithK8sPredicate(predicate.GatewayClassPredicate{ControllerName: cfg.GatewayCtlrName}),
			},
		},
		{
			objectType: &gatewayv1.Gateway{},
		},
		{
			objectType: &crdWithGVK,
			options: []controller.Option{
				controller.WithOnlyMetadata(),
				controller.WithK8sPredicate(
					predicate.AnnotationPredicate{Annotation: gatewayclass.BundleVersionAnnotation},
				),
			},
		},
	}

	ctx := ctlr.SetupSignalHandler()
	eventCh := make(chan interface{})

	for _, regCfg := range controllerRegCfgs {
		if err := controller.Register(
			ctx,
			regCfg.objectType,
			regCfg.objectType.GetObjectKind().GroupVersionKind().Kind,
			mgr,
			eventCh,
			regCfg.options...,
		); err != nil {
			return fmt.Errorf("cannot register controller for %T: %w", regCfg.objectType, err)
		}
	}

	partialObjectMetadataList := &metav1.PartialObjectMetadataList{}
	partialObjectMetadataList.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   apiext.GroupName,
			Version: "v1",
			Kind:    "CustomResourceDefinition",
		},
	)

	firstBatchPreparer := events.NewFirstEventBatchPreparerImpl(
		mgr.GetCache(),
		[]client.Object{
			&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: cfg.GatewayClassName}},
		},
		[]client.ObjectList{
			&gatewayv1.GatewayList{},
			partialObjectMetadataList,
		},
	)

	statusUpdater := status.NewUpdater(
		mgr.GetClient(),
		cfg.Logger.WithName("statusUpdater"),
	)

	handler := newEventHandler(
		cfg.GatewayClassName,
		statusUpdater,
		mgr.GetClient(),
		embeddedfiles.StaticModeDeploymentYAML,
		metav1.Now,
	)

	eventLoop := events.NewEventLoop(
		eventCh,
		cfg.Logger.WithName("eventLoop"),
		handler,
		firstBatchPreparer,
	)

	if err := mgr.Add(eventLoop); err != nil {
		return fmt.Errorf("cannot register event loop: %w", err)
	}

	cfg.Logger.Info("Starting manager")
	return mgr.Start(ctx)
}
