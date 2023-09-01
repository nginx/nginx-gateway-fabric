package static

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	k8spredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	nkgAPI "github.com/nginxinc/nginx-kubernetes-gateway/apis/v1alpha1"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/controller"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/controller/filter"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/controller/index"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/controller/predicate"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/events"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/framework/status"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/config"
	nkgmetrics "github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/metrics"
	ngxcfg "github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/nginx/config"
	ngxvalidation "github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/nginx/config/validation"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/nginx/file"
	ngxruntime "github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/nginx/runtime"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state/relationship"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state/resolver"
	"github.com/nginxinc/nginx-kubernetes-gateway/internal/mode/static/state/validation"
)

const (
	// clusterTimeout is a timeout for connections to the Kubernetes API
	clusterTimeout = 10 * time.Second
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(gatewayv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))
	utilruntime.Must(discoveryV1.AddToScheme(scheme))
	utilruntime.Must(nkgAPI.AddToScheme(scheme))
}

func StartManager(cfg config.Config) error {
	logger := cfg.Logger

	options := manager.Options{
		Scheme:  scheme,
		Logger:  logger,
		Metrics: getMetricsOptions(cfg.MetricsConfig),
	}

	eventCh := make(chan interface{})

	clusterCfg := ctlr.GetConfigOrDie()
	clusterCfg.Timeout = clusterTimeout

	mgr, err := manager.New(clusterCfg, options)
	if err != nil {
		return fmt.Errorf("cannot build runtime manager: %w", err)
	}

	recorderName := fmt.Sprintf("nginx-kubernetes-gateway-%s", cfg.GatewayClassName)
	recorder := mgr.GetEventRecorderFor(recorderName)
	logLevelSetter := newZapLogLevelSetter(cfg.AtomicLevel)

	// Note: for any new object type or a change to the existing one,
	// make sure to also update prepareFirstEventBatchPreparerArgs()
	type ctlrCfg struct {
		objectType client.Object
		options    []controller.Option
	}
	controllerRegCfgs := []ctlrCfg{
		{
			objectType: &gatewayv1beta1.GatewayClass{},
			options: []controller.Option{
				controller.WithK8sPredicate(predicate.GatewayClassPredicate{ControllerName: cfg.GatewayCtlrName}),
			},
		},
		{
			objectType: &gatewayv1beta1.Gateway{},
			options: func() []controller.Option {
				if cfg.GatewayNsName != nil {
					return []controller.Option{
						controller.WithNamespacedNameFilter(filter.CreateSingleResourceFilter(*cfg.GatewayNsName)),
					}
				}
				return nil
			}(),
		},
		{
			objectType: &gatewayv1beta1.HTTPRoute{},
		},
		{
			objectType: &apiv1.Service{},
			options: []controller.Option{
				controller.WithK8sPredicate(predicate.ServicePortsChangedPredicate{}),
			},
		},
		{
			objectType: &apiv1.Secret{},
		},
		{
			objectType: &discoveryV1.EndpointSlice{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				controller.WithFieldIndices(index.CreateEndpointSliceFieldIndices()),
			},
		},
		{
			objectType: &apiv1.Namespace{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.LabelChangedPredicate{}),
			},
		},
		{
			objectType: &gatewayv1beta1.ReferenceGrant{},
		},
	}

	controlConfigNSName := types.NamespacedName{
		Namespace: cfg.Namespace,
		Name:      cfg.ConfigName,
	}
	if cfg.ConfigName != "" {
		controllerRegCfgs = append(controllerRegCfgs,
			ctlrCfg{
				objectType: &nkgAPI.NginxGateway{},
				options: []controller.Option{
					controller.WithNamespacedNameFilter(filter.CreateSingleResourceFilter(controlConfigNSName)),
				},
			})
		if err := setInitialConfig(
			mgr.GetAPIReader(),
			logger, recorder,
			logLevelSetter,
			controlConfigNSName,
		); err != nil {
			return fmt.Errorf("error setting initial control plane configuration: %w", err)
		}
	}

	ctx := ctlr.SetupSignalHandler()

	for _, regCfg := range controllerRegCfgs {
		if err := controller.Register(
			ctx,
			regCfg.objectType,
			mgr,
			eventCh,
			regCfg.options...,
		); err != nil {
			return fmt.Errorf("cannot register controller for %T: %w", regCfg.objectType, err)
		}
	}

	// protectedPorts is the map of ports that may not be configured by a listener, and the name of what it is used for
	protectedPorts := map[int32]string{
		int32(cfg.MetricsConfig.Port): "MetricsPort",
	}

	processor := state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
		GatewayCtlrName:      cfg.GatewayCtlrName,
		GatewayClassName:     cfg.GatewayClassName,
		RelationshipCapturer: relationship.NewCapturerImpl(),
		Logger:               cfg.Logger.WithName("changeProcessor"),
		Validators: validation.Validators{
			HTTPFieldsValidator: ngxvalidation.HTTPValidator{},
		},
		EventRecorder:  recorder,
		Scheme:         scheme,
		ProtectedPorts: protectedPorts,
	})

	configGenerator := ngxcfg.NewGeneratorImpl()

	// Clear the configuration folders to ensure that no files are left over in case the control plane was restarted
	// (this assumes the folders are in a shared volume).
	removedPaths, err := file.ClearFolders(file.NewStdLibOSFileManager(), ngxcfg.ConfigFolders)
	for _, path := range removedPaths {
		logger.Info("removed configuration file", "path", path)
	}
	if err != nil {
		return fmt.Errorf("cannot clear NGINX configuration folders: %w", err)
	}

	nginxFileMgr := file.NewManagerImpl(logger.WithName("nginxFileManager"), file.NewStdLibOSFileManager())
	nginxRuntimeMgr := ngxruntime.NewManagerImpl()
	statusUpdater := status.NewUpdater(status.UpdaterConfig{
		GatewayCtlrName:          cfg.GatewayCtlrName,
		GatewayClassName:         cfg.GatewayClassName,
		Client:                   mgr.GetClient(),
		PodIP:                    cfg.PodIP,
		Logger:                   cfg.Logger.WithName("statusUpdater"),
		Clock:                    status.NewRealClock(),
		UpdateGatewayClassStatus: cfg.UpdateGatewayClassStatus,
	})

	eventHandler := newEventHandlerImpl(eventHandlerConfig{
		processor:           processor,
		serviceResolver:     resolver.NewServiceResolverImpl(mgr.GetClient()),
		generator:           configGenerator,
		logger:              cfg.Logger.WithName("eventHandler"),
		logLevelSetter:      logLevelSetter,
		nginxFileMgr:        nginxFileMgr,
		nginxRuntimeMgr:     nginxRuntimeMgr,
		statusUpdater:       statusUpdater,
		eventRecorder:       recorder,
		controlConfigNSName: controlConfigNSName,
	})

	objects, objectLists := prepareFirstEventBatchPreparerArgs(cfg.GatewayClassName, cfg.GatewayNsName)
	firstBatchPreparer := events.NewFirstEventBatchPreparerImpl(mgr.GetCache(), objects, objectLists)

	eventLoop := events.NewEventLoop(
		eventCh,
		cfg.Logger.WithName("eventLoop"),
		eventHandler,
		firstBatchPreparer)

	if err = mgr.Add(eventLoop); err != nil {
		return fmt.Errorf("cannot register event loop: %w", err)
	}

	// Ensure NGINX is running before registering metrics & starting the manager.
	if err := ngxruntime.EnsureNginxRunning(ctx); err != nil {
		return fmt.Errorf("NGINX is not running: %w", err)
	}

	if cfg.MetricsConfig.Enabled {
		if err := configureNginxMetrics(cfg.GatewayClassName); err != nil {
			return err
		}
	}

	logger.Info("Starting manager")
	return mgr.Start(ctx)
}

func prepareFirstEventBatchPreparerArgs(
	gcName string,
	gwNsName *types.NamespacedName,
) ([]client.Object, []client.ObjectList) {
	objects := []client.Object{
		&gatewayv1beta1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: gcName}},
	}
	objectLists := []client.ObjectList{
		&apiv1.ServiceList{},
		&apiv1.SecretList{},
		&apiv1.NamespaceList{},
		&discoveryV1.EndpointSliceList{},
		&gatewayv1beta1.HTTPRouteList{},
		&gatewayv1beta1.ReferenceGrantList{},
	}

	if gwNsName == nil {
		objectLists = append(objectLists, &gatewayv1beta1.GatewayList{})
	} else {
		objects = append(
			objects,
			&gatewayv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: gwNsName.Name, Namespace: gwNsName.Namespace}},
		)
	}

	return objects, objectLists
}

func setInitialConfig(
	reader client.Reader,
	logger logr.Logger,
	eventRecorder record.EventRecorder,
	logLevelSetter ZapLogLevelSetter,
	configName types.NamespacedName,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var config nkgAPI.NginxGateway
	if err := reader.Get(ctx, configName, &config); err != nil {
		return err
	}

	// status is not updated until the status updater's cache is started and the
	// resource is processed by the controller
	return updateControlPlane(&config, logger, eventRecorder, configName, logLevelSetter)
}

func configureNginxMetrics(gatewayClassName string) error {
	constLabels := map[string]string{"class": gatewayClassName}
	ngxCollector, err := nkgmetrics.NewNginxMetricsCollector(constLabels)
	if err != nil {
		return fmt.Errorf("cannot get NGINX metrics: %w", err)
	}
	return metrics.Registry.Register(ngxCollector)
}

func getMetricsOptions(cfg config.MetricsConfig) metricsserver.Options {
	metricsOptions := metricsserver.Options{BindAddress: "0"}

	if cfg.Enabled {
		if cfg.Secure {
			metricsOptions.SecureServing = true
		}
		metricsOptions.BindAddress = fmt.Sprintf(":%v", cfg.Port)
	}

	return metricsOptions
}
