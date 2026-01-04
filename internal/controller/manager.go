package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"google.golang.org/grpc"
	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authentication/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcfg "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	k8spredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	"sigs.k8s.io/gateway-api/pkg/consts"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/licensing"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/metrics/collectors"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	agentgrpc "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent/grpc"
	ngxcfg "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/clientsettings"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/observability"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/upstreamsettings"
	ngxvalidation "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/provisioner"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/resolver"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/telemetry"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller/filter"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller/index"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller/predicate"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/runnables"
	ngftypes "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

const (
	// clusterTimeout is a timeout for connections to the Kubernetes API.
	clusterTimeout = 10 * time.Second
	// the following are the names of data fields within NGINX Plus related Secrets.
	plusLicenseField    = "license.jwt"
	plusCAField         = "ca.crt"
	plusClientCertField = "tls.crt"
	plusClientKeyField  = "tls.key"
	grpcServerPort      = 8443
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(gatewayv1beta1.Install(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(gatewayv1alpha2.Install(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))
	utilruntime.Must(discoveryV1.AddToScheme(scheme))
	utilruntime.Must(ngfAPIv1alpha1.AddToScheme(scheme))
	utilruntime.Must(ngfAPIv1alpha2.AddToScheme(scheme))
	utilruntime.Must(apiext.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(autoscalingv2.AddToScheme(scheme))
	utilruntime.Must(authv1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))
	utilruntime.Must(inference.Install(scheme))
}

func StartManager(cfg config.Config) error {
	healthChecker := newGraphBuiltHealthChecker()
	mgr, err := createManager(cfg, healthChecker)
	if err != nil {
		return fmt.Errorf("cannot build runtime manager: %w", err)
	}

	recorderName := fmt.Sprintf("nginx-gateway-fabric-%s", cfg.GatewayClassName)
	recorder := mgr.GetEventRecorderFor(recorderName)

	logLevelSetter := newMultiLogLevelSetter(newZapLogLevelSetter(cfg.AtomicLevel))

	ctx := ctlr.SetupSignalHandler()

	eventCh := make(chan interface{})
	controlConfigNSName := types.NamespacedName{
		Namespace: cfg.GatewayPodConfig.Namespace,
		Name:      cfg.ConfigName,
	}
	if err := registerControllers(ctx, cfg, mgr, recorder, logLevelSetter, eventCh, controlConfigNSName); err != nil {
		return err
	}

	mustExtractGVK := kinds.NewMustExtractGKV(scheme)

	genericValidator := ngxvalidation.GenericValidator{}
	policyManager := createPolicyManager(mustExtractGVK, genericValidator, cfg.Plus)

	plusSecrets, err := createPlusSecretMetadata(cfg, mgr.GetAPIReader())
	if err != nil {
		return err
	}

	processor := state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
		GatewayCtlrName:  cfg.GatewayCtlrName,
		GatewayClassName: cfg.GatewayClassName,
		Logger:           cfg.Logger.WithName("changeProcessor"),
		Validators: validation.Validators{
			HTTPFieldsValidator: ngxvalidation.HTTPValidator{},
			GenericValidator:    genericValidator,
			PolicyValidator:     policyManager,
		},
		EventRecorder:  recorder,
		MustExtractGVK: mustExtractGVK,
		PlusSecrets:    plusSecrets,
		FeatureFlags: graph.FeatureFlags{
			Plus:         cfg.Plus,
			Experimental: cfg.ExperimentalFeatures,
		},
	})

	var handlerCollector handlerMetricsCollector = collectors.NewControllerNoopCollector()

	if cfg.MetricsConfig.Enabled {
		constLabels := map[string]string{"class": cfg.GatewayClassName}

		handlerCollector = collectors.NewControllerCollector(constLabels)
		handlerCollector, ok := handlerCollector.(prometheus.Collector)
		if !ok {
			return fmt.Errorf("handlerCollector is not a prometheus.Collector: %w", status.ErrFailedAssert)
		}

		metrics.Registry.MustRegister(handlerCollector)
	}

	statusUpdater := status.NewUpdater(
		mgr.GetClient(),
		cfg.Logger.WithName("statusUpdater"),
	)

	groupStatusUpdater := status.NewLeaderAwareGroupUpdater(statusUpdater)
	deployCtxCollector := licensing.NewDeploymentContextCollector(licensing.DeploymentContextCollectorConfig{
		K8sClientReader: mgr.GetAPIReader(),
		PodUID:          cfg.GatewayPodConfig.UID,
		Logger:          cfg.Logger.WithName("deployCtxCollector"),
	})

	statusQueue := status.NewQueue()
	resetConnChan := make(chan struct{})
	nginxUpdater := agent.NewNginxUpdater(
		cfg.Logger.WithName("nginxUpdater"),
		mgr.GetAPIReader(),
		statusQueue,
		resetConnChan,
		cfg.Plus,
	)

	tokenAudience := fmt.Sprintf(
		"%s.%s.svc",
		cfg.GatewayPodConfig.ServiceName,
		cfg.GatewayPodConfig.Namespace,
	)

	grpcServer := agentgrpc.NewServer(
		cfg.Logger.WithName("agentGRPCServer"),
		grpcServerPort,
		[]func(*grpc.Server){
			nginxUpdater.CommandService.Register,
			nginxUpdater.FileService.Register,
		},
		mgr.GetClient(),
		tokenAudience,
		resetConnChan,
	)

	if err = mgr.Add(&runnables.LeaderOrNonLeader{Runnable: grpcServer}); err != nil {
		return fmt.Errorf("cannot register grpc server: %w", err)
	}

	nginxProvisioner, provLoop, err := provisioner.NewNginxProvisioner(
		ctx,
		mgr,
		provisioner.Config{
			DeploymentStore:                nginxUpdater.NginxDeployments,
			StatusQueue:                    statusQueue,
			Logger:                         cfg.Logger.WithName("provisioner"),
			EventRecorder:                  recorder,
			GatewayPodConfig:               &cfg.GatewayPodConfig,
			GCName:                         cfg.GatewayClassName,
			AgentTLSSecretName:             cfg.AgentTLSSecretName,
			NGINXSCCName:                   cfg.NGINXSCCName,
			Plus:                           cfg.Plus,
			NginxDockerSecretNames:         cfg.NginxDockerSecretNames,
			PlusUsageConfig:                &cfg.UsageReportConfig,
			NginxOneConsoleTelemetryConfig: cfg.NginxOneConsoleTelemetryConfig,
			InferenceExtension:             cfg.InferenceExtension,
			EndpointPickerDisableTLS:       cfg.EndpointPickerDisableTLS,
			EndpointPickerTLSSkipVerify:    cfg.EndpointPickerTLSSkipVerify,
		},
	)
	if err != nil {
		return fmt.Errorf("error building provisioner: %w", err)
	}

	if err := mgr.Add(&runnables.LeaderOrNonLeader{Runnable: provLoop}); err != nil {
		return fmt.Errorf("cannot register provisioner event loop: %w", err)
	}

	eventHandler := newEventHandlerImpl(eventHandlerConfig{
		ctx:              ctx,
		nginxUpdater:     nginxUpdater,
		nginxProvisioner: nginxProvisioner,
		metricsCollector: handlerCollector,
		statusUpdater:    groupStatusUpdater,
		processor:        processor,
		serviceResolver:  resolver.NewServiceResolverImpl(mgr.GetClient()),
		generator: ngxcfg.NewGeneratorImpl(
			cfg.Plus,
			&cfg.UsageReportConfig,
			cfg.Logger.WithName("generator"),
		),
		k8sClient:               mgr.GetClient(),
		k8sReader:               mgr.GetAPIReader(),
		logger:                  cfg.Logger.WithName("eventHandler"),
		logLevelSetter:          logLevelSetter,
		eventRecorder:           recorder,
		deployCtxCollector:      deployCtxCollector,
		graphBuiltHealthChecker: healthChecker,
		gatewayPodConfig:        cfg.GatewayPodConfig,
		controlConfigNSName:     controlConfigNSName,
		gatewayCtlrName:         cfg.GatewayCtlrName,
		gatewayInstanceName:     cfg.GatewayPodConfig.InstanceName,
		gatewayClassName:        cfg.GatewayClassName,
		plus:                    cfg.Plus,
		statusQueue:             statusQueue,
		nginxDeployments:        nginxUpdater.NginxDeployments,
		inferenceExtension:      cfg.InferenceExtension,
	})

	objects, objectLists := prepareFirstEventBatchPreparerArgs(cfg)

	firstBatchPreparer := events.NewFirstEventBatchPreparerImpl(mgr.GetCache(), objects, objectLists)
	eventLoop := events.NewEventLoop(
		eventCh,
		cfg.Logger.WithName("eventLoop"),
		eventHandler,
		firstBatchPreparer,
	)

	if err = mgr.Add(&runnables.LeaderOrNonLeader{Runnable: eventLoop}); err != nil {
		return fmt.Errorf("cannot register event loop: %w", err)
	}

	if err = mgr.Add(runnables.NewCallFunctionsAfterBecameLeader([]func(context.Context){
		groupStatusUpdater.Enable,
		nginxProvisioner.Enable,
		eventHandler.enable,
	})); err != nil {
		return fmt.Errorf("cannot register functions that get called after Pod becomes leader: %w", err)
	}

	if cfg.ProductTelemetryConfig.Enabled {
		dataCollector := telemetry.NewDataCollectorImpl(telemetry.DataCollectorConfig{
			K8sClientReader:     mgr.GetAPIReader(),
			GraphGetter:         processor,
			ConfigurationGetter: eventHandler,
			Version:             cfg.GatewayPodConfig.Version,
			PodNSName: types.NamespacedName{
				Namespace: cfg.GatewayPodConfig.Namespace,
				Name:      cfg.GatewayPodConfig.Name,
			},
			ImageSource:               cfg.ImageSource,
			Flags:                     cfg.Flags,
			NginxOneConsoleConnection: cfg.NginxOneConsoleTelemetryConfig.DataplaneKeySecretName != "",
		})

		job, err := createTelemetryJob(cfg, dataCollector, healthChecker.getReadyCh())
		if err != nil {
			return fmt.Errorf("cannot create telemetry job: %w", err)
		}

		if err = mgr.Add(job); err != nil {
			return fmt.Errorf("cannot register telemetry job: %w", err)
		}
	}

	cfg.Logger.Info("Starting manager")
	go func() {
		<-ctx.Done()
		cfg.Logger.Info("Shutting down")
	}()

	return mgr.Start(ctx)
}

func createPolicyManager(
	mustExtractGVK kinds.MustExtractGVK,
	validator validation.GenericValidator,
	plusEnabled bool,
) *policies.CompositeValidator {
	cfgs := []policies.ManagerConfig{
		{
			GVK:       mustExtractGVK(&ngfAPIv1alpha1.ClientSettingsPolicy{}),
			Validator: clientsettings.NewValidator(validator),
		},
		{
			GVK:       mustExtractGVK(&ngfAPIv1alpha2.ObservabilityPolicy{}),
			Validator: observability.NewValidator(validator),
		},
		{
			GVK:       mustExtractGVK(&ngfAPIv1alpha1.UpstreamSettingsPolicy{}),
			Validator: upstreamsettings.NewValidator(validator, plusEnabled),
		},
	}

	return policies.NewManager(mustExtractGVK, cfgs...)
}

func createManager(cfg config.Config, healthChecker *graphBuiltHealthChecker) (manager.Manager, error) {
	options := manager.Options{
		Scheme:  scheme,
		Logger:  cfg.Logger.V(1),
		Metrics: getMetricsOptions(cfg.MetricsConfig),
		// Note: when the leadership is lost, the manager will return an error in the Start() method.
		// However, it will not wait for any Runnable it starts to finish, meaning any in-progress operations
		// might get terminated half-way.
		LeaderElection:          cfg.LeaderElection.Enabled,
		LeaderElectionNamespace: cfg.GatewayPodConfig.Namespace,
		LeaderElectionID:        cfg.LeaderElection.LockName,
		// We're not enabling LeaderElectionReleaseOnCancel because when the Manager stops gracefully, it waits
		// for all started Runnables (including Leader-only ones) to finish. Otherwise, the new leader might start
		// running Leader-only Runnables before the old leader has finished running them.
		// See the doc comment for the LeaderElectionReleaseOnCancel for more details.
		LeaderElectionReleaseOnCancel: false,
		Controller: ctrlcfg.Controller{
			// All of our controllers still need to work in case of non-leader pods
			NeedLeaderElection: helpers.GetPointer(false),
		},
	}

	if cfg.HealthConfig.Enabled {
		options.HealthProbeBindAddress = fmt.Sprintf(":%d", cfg.HealthConfig.Port)
	}

	clusterCfg := ctlr.GetConfigOrDie()
	clusterCfg.Timeout = clusterTimeout

	mgr, err := manager.New(clusterCfg, options)
	if err != nil {
		return nil, err
	}

	if cfg.HealthConfig.Enabled {
		if err := mgr.AddReadyzCheck("readyz", healthChecker.readyCheck); err != nil {
			return nil, fmt.Errorf("error adding ready check: %w", err)
		}
	}

	// Add an indexer to get pods by their IP address. This is used when validating that an agent
	// connection is coming from the right place.
	var podIPIndexFunc client.IndexerFunc = index.PodIPIndexFunc
	if err := controller.AddIndex(
		context.Background(),
		mgr.GetFieldIndexer(),
		&apiv1.Pod{},
		"status.podIP",
		podIPIndexFunc,
	); err != nil {
		return nil, fmt.Errorf("error adding pod IP indexer: %w", err)
	}

	return mgr, nil
}

func registerControllers(
	ctx context.Context,
	cfg config.Config,
	mgr manager.Manager,
	recorder record.EventRecorder,
	logLevelSetter logLevelSetter,
	eventCh chan interface{},
	controlConfigNSName types.NamespacedName,
) error {
	type ctlrCfg struct {
		name       string
		objectType ngftypes.ObjectType
		options    []controller.Option
	}

	crdWithGVK := apiext.CustomResourceDefinition{}
	crdWithGVK.SetGroupVersionKind(
		schema.GroupVersionKind{Group: apiext.GroupName, Version: "v1", Kind: "CustomResourceDefinition"},
	)

	// Note: for any new object type or a change to the existing one,
	// make sure to also update prepareFirstEventBatchPreparerArgs()
	controllerRegCfgs := []ctlrCfg{
		{
			objectType: &gatewayv1.GatewayClass{},
			options: []controller.Option{
				controller.WithK8sPredicate(
					k8spredicate.And(
						k8spredicate.GenerationChangedPredicate{},
						predicate.GatewayClassPredicate{ControllerName: cfg.GatewayCtlrName},
					),
				),
			},
		},
		{
			objectType: &gatewayv1.Gateway{},
			options: func() []controller.Option {
				options := []controller.Option{
					controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				}
				return options
			}(),
		},
		{
			objectType: &gatewayv1.HTTPRoute{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
		{
			objectType: &gatewayv1.BackendTLSPolicy{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
		{
			// FIXME(ciarams87): If possible, use only metadata predicate
			// https://github.com/nginx/nginx-gateway-fabric/issues/1545
			objectType: &apiv1.ConfigMap{},
		},
		{
			objectType: &apiv1.Service{},
			name:       "user-service", // unique controller names are needed and we have multiple Service ctlrs
			options: []controller.Option{
				controller.WithK8sPredicate(predicate.ServiceChangedPredicate{}),
			},
		},
		{
			objectType: &apiv1.Secret{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.ResourceVersionChangedPredicate{}),
			},
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
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
		{
			objectType: &crdWithGVK,
			options: []controller.Option{
				controller.WithOnlyMetadata(),
				controller.WithK8sPredicate(
					predicate.AnnotationPredicate{Annotation: consts.BundleVersionAnnotation},
				),
			},
		},
		{
			objectType: &ngfAPIv1alpha2.NginxProxy{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
		{
			objectType: &gatewayv1.GRPCRoute{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
		{
			objectType: &ngfAPIv1alpha1.ClientSettingsPolicy{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
		{
			objectType: &ngfAPIv1alpha2.ObservabilityPolicy{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
		{
			objectType: &ngfAPIv1alpha1.UpstreamSettingsPolicy{},
			options: []controller.Option{
				controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
			},
		},
	}

	if cfg.ExperimentalFeatures {
		gwExpFeatures := []ctlrCfg{
			{
				objectType: &gatewayv1alpha2.TLSRoute{},
				options: []controller.Option{
					controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				},
			},
			{
				objectType: &gatewayv1alpha2.TCPRoute{},
				options: []controller.Option{
					controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				},
			},
			{
				objectType: &gatewayv1alpha2.UDPRoute{},
				options: []controller.Option{
					controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				},
			},
		}
		controllerRegCfgs = append(controllerRegCfgs, gwExpFeatures...)
	}

	if cfg.InferenceExtension {
		inferenceExt := []ctlrCfg{
			{
				objectType: &inference.InferencePool{},
				options: []controller.Option{
					controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				},
			},
		}
		controllerRegCfgs = append(controllerRegCfgs, inferenceExt...)
	}

	if cfg.ConfigName != "" {
		controllerRegCfgs = append(controllerRegCfgs,
			ctlrCfg{
				objectType: &ngfAPIv1alpha1.NginxGateway{},
				options: []controller.Option{
					controller.WithNamespacedNameFilter(filter.CreateSingleResourceFilter(controlConfigNSName)),
					controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				},
			})
		if err := setInitialConfig(
			mgr.GetAPIReader(),
			cfg.Logger,
			recorder,
			logLevelSetter,
			controlConfigNSName,
		); err != nil {
			return fmt.Errorf("error setting initial control plane configuration: %w", err)
		}
	}

	if cfg.SnippetsFilters {
		controllerRegCfgs = append(controllerRegCfgs,
			ctlrCfg{
				objectType: &ngfAPIv1alpha1.SnippetsFilter{},
				options: []controller.Option{
					controller.WithK8sPredicate(k8spredicate.GenerationChangedPredicate{}),
				},
			},
		)
	}

	for _, regCfg := range controllerRegCfgs {
		name := regCfg.objectType.GetObjectKind().GroupVersionKind().Kind
		if regCfg.name != "" {
			name = regCfg.name
		}

		if err := controller.Register(
			ctx,
			regCfg.objectType,
			name,
			mgr,
			eventCh,
			regCfg.options...,
		); err != nil {
			return fmt.Errorf("cannot register controller for %T: %w", regCfg.objectType, err)
		}
	}
	return nil
}

func createPlusSecretMetadata(
	cfg config.Config,
	reader client.Reader,
) (map[types.NamespacedName][]graph.PlusSecretFile, error) {
	plusSecrets := make(map[types.NamespacedName][]graph.PlusSecretFile)
	if cfg.Plus {
		jwtSecretName := types.NamespacedName{
			Namespace: cfg.GatewayPodConfig.Namespace,
			Name:      cfg.UsageReportConfig.SecretName,
		}

		if err := validateSecret(reader, jwtSecretName, plusLicenseField); err != nil {
			return nil, err
		}

		jwtSecretCfg := graph.PlusSecretFile{
			FieldName: plusLicenseField,
			Type:      graph.PlusReportJWTToken,
		}

		plusSecrets[jwtSecretName] = []graph.PlusSecretFile{jwtSecretCfg}

		if cfg.UsageReportConfig.CASecretName != "" {
			caSecretName := types.NamespacedName{
				Namespace: cfg.GatewayPodConfig.Namespace,
				Name:      cfg.UsageReportConfig.CASecretName,
			}

			if err := validateSecret(reader, caSecretName, plusCAField); err != nil {
				return nil, err
			}

			caSecretCfg := graph.PlusSecretFile{
				FieldName: plusCAField,
				Type:      graph.PlusReportCACertificate,
			}

			plusSecrets[caSecretName] = []graph.PlusSecretFile{caSecretCfg}
		}

		if cfg.UsageReportConfig.ClientSSLSecretName != "" {
			clientSSLSecretName := types.NamespacedName{
				Namespace: cfg.GatewayPodConfig.Namespace,
				Name:      cfg.UsageReportConfig.ClientSSLSecretName,
			}

			if err := validateSecret(reader, clientSSLSecretName, plusClientCertField, plusClientKeyField); err != nil {
				return nil, err
			}

			clientSSLCertCfg := graph.PlusSecretFile{
				FieldName: plusClientCertField,
				Type:      graph.PlusReportClientSSLCertificate,
			}

			clientSSLKeyCfg := graph.PlusSecretFile{
				FieldName: plusClientKeyField,
				Type:      graph.PlusReportClientSSLKey,
			}

			plusSecrets[clientSSLSecretName] = []graph.PlusSecretFile{clientSSLCertCfg, clientSSLKeyCfg}
		}
	}

	return plusSecrets, nil
}

func validateSecret(reader client.Reader, nsName types.NamespacedName, fields ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var secret apiv1.Secret
	if err := reader.Get(ctx, nsName, &secret); err != nil {
		return fmt.Errorf("error getting %q Secret: %w", nsName.Name, err)
	}

	for _, field := range fields {
		if _, ok := secret.Data[field]; !ok {
			return fmt.Errorf("secret %q does not have expected field %q", nsName.Name, field)
		}
	}

	return nil
}

// 10 min jitter is enough per telemetry destination recommendation
// For the default period of 24 hours, jitter will be 10min /(24*60)min  = 0.0069.
const telemetryJitterFactor = 10.0 / (24 * 60) // added jitter is bound by jitterFactor * period

func createTelemetryJob(
	cfg config.Config,
	dataCollector telemetry.DataCollector,
	readyCh <-chan struct{},
) (*runnables.Leader, error) {
	logger := cfg.Logger.WithName("telemetryJob")

	var exporter telemetry.Exporter

	if cfg.ProductTelemetryConfig.Endpoint != "" {
		errorHandler := tel.NewErrorHandler()

		options := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.ProductTelemetryConfig.Endpoint),
		}
		if cfg.ProductTelemetryConfig.EndpointInsecure {
			options = append(options, otlptracegrpc.WithInsecure())
		}

		var err error
		exporter, err = tel.NewExporter(
			tel.ExporterConfig{
				SpanProvider: tel.CreateOTLPSpanProvider(options...),
			},
			tel.WithGlobalOTelLogger(logger.WithName("otel")),
			tel.WithGlobalOTelErrorHandler(errorHandler),
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create telemetry exporter: %w", err)
		}
	} else {
		exporter = telemetry.NewLoggingExporter(cfg.Logger.WithName("telemetryExporter").V(1 /* debug */))
	}

	return &runnables.Leader{
		Runnable: runnables.NewCronJob(
			runnables.CronJobConfig{
				Worker:       telemetry.CreateTelemetryJobWorker(logger, exporter, dataCollector),
				Logger:       logger,
				Period:       cfg.ProductTelemetryConfig.ReportPeriod,
				JitterFactor: telemetryJitterFactor,
				ReadyCh:      readyCh,
			},
		),
	}, nil
}

func prepareFirstEventBatchPreparerArgs(cfg config.Config) ([]client.Object, []client.ObjectList) {
	objects := []client.Object{
		&gatewayv1.GatewayClass{ObjectMeta: metav1.ObjectMeta{Name: cfg.GatewayClassName}},
	}

	partialObjectMetadataList := &metav1.PartialObjectMetadataList{}
	partialObjectMetadataList.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   apiext.GroupName,
			Version: "v1",
			Kind:    "CustomResourceDefinition",
		},
	)

	objectLists := []client.ObjectList{
		&apiv1.ServiceList{},
		&apiv1.SecretList{},
		&apiv1.NamespaceList{},
		&discoveryV1.EndpointSliceList{},
		&gatewayv1.HTTPRouteList{},
		&gatewayv1.BackendTLSPolicyList{},
		&apiv1.ConfigMapList{},
		&gatewayv1beta1.ReferenceGrantList{},
		&ngfAPIv1alpha2.NginxProxyList{},
		&gatewayv1.GRPCRouteList{},
		&ngfAPIv1alpha1.ClientSettingsPolicyList{},
		&ngfAPIv1alpha2.ObservabilityPolicyList{},
		&ngfAPIv1alpha1.UpstreamSettingsPolicyList{},
		partialObjectMetadataList,
	}

	if cfg.ExperimentalFeatures {
		objectLists = append(
			objectLists,
			&gatewayv1alpha2.TLSRouteList{},
			&gatewayv1alpha2.TCPRouteList{},
			&gatewayv1alpha2.UDPRouteList{},
		)
	}

	if cfg.InferenceExtension {
		objectLists = append(objectLists, &inference.InferencePoolList{})
	}

	if cfg.SnippetsFilters {
		objectLists = append(
			objectLists,
			&ngfAPIv1alpha1.SnippetsFilterList{},
		)
	}

	objectLists = append(objectLists, &gatewayv1.GatewayList{})

	return objects, objectLists
}

func setInitialConfig(
	reader client.Reader,
	logger logr.Logger,
	eventRecorder record.EventRecorder,
	logLevelSetter logLevelSetter,
	configName types.NamespacedName,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var conf ngfAPIv1alpha1.NginxGateway
	// Polling to wait for CRD to exist if the Deployment is created first.
	if err := wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			if err := reader.Get(ctx, configName, &conf); err != nil {
				if !apierrors.IsNotFound(err) {
					return false, err
				}
				return false, nil
			}
			return true, nil
		},
	); err != nil {
		return fmt.Errorf("NginxGateway %s not found: %w", configName, err)
	}

	// status is not updated until the status updater's cache is started and the
	// resource is processed by the controller
	return updateControlPlane(&conf, logger, eventRecorder, configName, logLevelSetter)
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
