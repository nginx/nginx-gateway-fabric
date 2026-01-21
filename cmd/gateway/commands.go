package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctlrZap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	ngxConfig "github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/file"
)

// These flags are shared by multiple commands.
const (
	domain                = "gateway.nginx.org"
	gatewayClassFlag      = "gatewayclass"
	gatewayClassNameUsage = `The name of the GatewayClass resource. ` +
		`Every NGINX Gateway Fabric must have a unique corresponding GatewayClass resource.`
	gatewayCtlrNameFlag     = "gateway-ctlr-name"
	gatewayCtlrNameUsageFmt = `The name of the Gateway controller. ` +
		`The controller name must be of the form: DOMAIN/PATH. The controller's domain is '%s'`
	plusFlag = "nginx-plus"

	serverTLSSecret                 = "server-tls"
	agentTLSSecret                  = "agent-tls"
	nginxOneTelemetryEndpointHost   = "agent.connect.nginx.com"
	endpointPickerDisableTLSFlag    = "endpoint-picker-disable-tls"
	endpointPickerTLSSkipVerifyFlag = "endpoint-picker-tls-skip-verify"
)

// usageReportParams holds the parameters for building the usage report configuration for PLUS.
type usageReportParams struct {
	SecretName           stringValidatingValue
	ClientSSLSecretName  stringValidatingValue
	CASecretName         stringValidatingValue
	Endpoint             stringValidatingValue
	Resolver             stringValidatingValue
	SkipVerify           bool
	EnforceInitialReport bool
}

func createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "gateway",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	return rootCmd
}

func createControllerCommand() *cobra.Command {
	// flag names
	const (
		configFlag                          = "config"
		serviceFlag                         = "service"
		agentTLSSecretFlag                  = "agent-tls-secret"
		nginxOneDataplaneKeySecretFlag      = "nginx-one-dataplane-key-secret" //nolint:gosec // not credentials
		nginxOneTelemetryEndpointHostFlag   = "nginx-one-telemetry-endpoint-host"
		nginxOneTelemetryEndpointPortFlag   = "nginx-one-telemetry-endpoint-port"
		nginxOneTLSSkipVerifyFlag           = "nginx-one-tls-skip-verify"
		metricsDisableFlag                  = "metrics-disable"
		metricsSecureFlag                   = "metrics-secure-serving"
		metricsPortFlag                     = "metrics-port"
		healthDisableFlag                   = "health-disable"
		healthPortFlag                      = "health-port"
		leaderElectionDisableFlag           = "leader-election-disable"
		leaderElectionLockNameFlag          = "leader-election-lock-name"
		productTelemetryDisableFlag         = "product-telemetry-disable"
		gwAPIExperimentalFlag               = "gateway-api-experimental-features"
		gwAPIInferenceExtensionFlag         = "gateway-api-inference-extension"
		nginxDockerSecretFlag               = "nginx-docker-secret" //nolint:gosec // not credentials
		usageReportSecretFlag               = "usage-report-secret"
		usageReportEndpointFlag             = "usage-report-endpoint"
		usageReportResolverFlag             = "usage-report-resolver"
		usageReportSkipVerifyFlag           = "usage-report-skip-verify"
		usageReportClientSSLSecretFlag      = "usage-report-client-ssl-secret" //nolint:gosec // not credentials
		usageReportCASecretFlag             = "usage-report-ca-secret"         //nolint:gosec // not credentials
		usageReportEnforceInitialReportFlag = "usage-report-enforce-initial-report"
		snippetsFiltersFlag                 = "snippets-filters"
		snippetsFlag                        = "snippets"
		nginxSCCFlag                        = "nginx-scc"
		watchNamespacesFlag                 = "watch-namespaces"
	)

	// flag values
	var (
		gatewayCtlrName = stringValidatingValue{
			validator: validateGatewayControllerName,
		}

		gatewayClassName = stringValidatingValue{
			validator: validateResourceName,
		}

		configName = stringValidatingValue{
			validator: validateResourceName,
		}
		serviceName = stringValidatingValue{
			validator: validateResourceName,
		}
		agentTLSSecretName = stringValidatingValue{
			validator: validateResourceName,
			value:     agentTLSSecret,
		}
		nginxOneConsoleDataplaneKeySecretName = stringValidatingValue{
			validator: validateResourceName,
		}
		nginxOneConsoleTelemetryEndpointHost = stringValidatingValue{
			validator: validateResourceName,
			value:     nginxOneTelemetryEndpointHost,
		}
		nginxOneConsoleTelemetryEndpointPort = intValidatingValue{
			validator: validateAnyPort,
			value:     443,
		}
		nginxOneConsoleTLSSkipVerify bool
		nginxSCCName                 = stringValidatingValue{
			validator: validateResourceName,
		}
		disableMetrics    bool
		metricsSecure     bool
		metricsListenPort = intValidatingValue{
			validator: validatePort,
			value:     9113,
		}
		disableHealth    bool
		healthListenPort = intValidatingValue{
			validator: validatePort,
			value:     8081,
		}

		disableLeaderElection  bool
		leaderElectionLockName = stringValidatingValue{
			validator: validateResourceName,
			value:     "nginx-gateway-leader-election-lock",
		}

		gwExperimentalFeatures bool
		gwInferenceExtension   bool

		disableProductTelemetry bool

		snippetsFilters bool
		snippets        bool

		plus               bool
		nginxDockerSecrets = stringSliceValidatingValue{
			validator: validateResourceName,
		}

		endpointPickerDisableTLS    bool
		endpointPickerTLSSkipVerify = true

		watchNamespaces = stringSliceValidatingValue{
			validator: validateResourceName,
		}
	)

	usageReportParams := usageReportParams{
		SecretName: stringValidatingValue{
			validator: validateResourceName,
			value:     "nplus-license",
		},
		Endpoint: stringValidatingValue{
			validator: validateEndpointOptionalPort,
		},
		Resolver: stringValidatingValue{
			validator: validateEndpointOptionalPort,
		},
		ClientSSLSecretName: stringValidatingValue{
			validator: validateResourceName,
		},
		CASecretName: stringValidatingValue{
			validator: validateResourceName,
		},
	}

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Run the NGINX Gateway Fabric control plane",
		RunE: func(cmd *cobra.Command, _ []string) error {
			atom := zap.NewAtomicLevel()

			logger := ctlrZap.New(ctlrZap.Level(atom))
			klog.SetLogger(logger)

			commit, date, dirty := getBuildInfo()
			logger.Info(
				"Starting the NGINX Gateway Fabric control plane",
				"version", version,
				"commit", commit,
				"date", date,
				"dirty", dirty,
			)
			log.SetLogger(logger)

			if err := ensureNoPortCollisions(metricsListenPort.value, healthListenPort.value); err != nil {
				return fmt.Errorf("error validating ports: %w", err)
			}

			imageSource := os.Getenv("BUILD_AGENT")
			if imageSource != "gha" && imageSource != "local" {
				imageSource = "unknown"
			}

			period, err := time.ParseDuration(telemetryReportPeriod)
			if err != nil {
				return fmt.Errorf("error parsing telemetry report period: %w", err)
			}

			if telemetryEndpoint != "" {
				if err := validateEndpoint(telemetryEndpoint); err != nil {
					return fmt.Errorf("error validating telemetry endpoint: %w", err)
				}
			}

			telemetryEndpointInsecure, err := strconv.ParseBool(telemetryEndpointInsecure)
			if err != nil {
				return fmt.Errorf("error parsing telemetry endpoint insecure: %w", err)
			}

			var usageReportConfig config.UsageReportConfig
			if plus {
				usageReportConfig, err = buildUsageReportConfig(usageReportParams)
				if err != nil {
					return err
				}
			}

			flagKeys, flagValues := parseFlags(cmd.Flags())

			podConfig, err := createGatewayPodConfig(version, serviceName.value)
			if err != nil {
				return fmt.Errorf("error creating gateway pod config: %w", err)
			}

			conf := config.Config{
				GatewayCtlrName:  gatewayCtlrName.value,
				ConfigName:       configName.String(),
				Logger:           logger,
				AtomicLevel:      atom,
				GatewayClassName: gatewayClassName.value,
				GatewayPodConfig: podConfig,
				HealthConfig: config.HealthConfig{
					Enabled: !disableHealth,
					Port:    healthListenPort.value,
				},
				MetricsConfig: config.MetricsConfig{
					Enabled: !disableMetrics,
					Port:    metricsListenPort.value,
					Secure:  metricsSecure,
				},
				LeaderElection: config.LeaderElectionConfig{
					Enabled:  !disableLeaderElection,
					LockName: leaderElectionLockName.String(),
					Identity: podConfig.Name,
				},
				UsageReportConfig: usageReportConfig,
				ProductTelemetryConfig: config.ProductTelemetryConfig{
					ReportPeriod:     period,
					Enabled:          !disableProductTelemetry,
					Endpoint:         telemetryEndpoint,
					EndpointInsecure: telemetryEndpointInsecure,
				},
				Plus:                 plus,
				ExperimentalFeatures: gwExperimentalFeatures,
				InferenceExtension:   gwInferenceExtension,
				ImageSource:          imageSource,
				Flags: config.Flags{
					Names:  flagKeys,
					Values: flagValues,
				},
				SnippetsFilters:        snippetsFilters,
				Snippets:               snippets,
				NginxDockerSecretNames: nginxDockerSecrets.values,
				AgentTLSSecretName:     agentTLSSecretName.value,
				NGINXSCCName:           nginxSCCName.value,
				NginxOneConsoleTelemetryConfig: config.NginxOneConsoleTelemetryConfig{
					DataplaneKeySecretName: nginxOneConsoleDataplaneKeySecretName.value,
					EndpointHost:           nginxOneConsoleTelemetryEndpointHost.value,
					EndpointPort:           nginxOneConsoleTelemetryEndpointPort.value,
					EndpointTLSSkipVerify:  nginxOneConsoleTLSSkipVerify,
				},
				EndpointPickerDisableTLS:    endpointPickerDisableTLS,
				EndpointPickerTLSSkipVerify: endpointPickerTLSSkipVerify,
				WatchNamespaces:             watchNamespaces.values,
			}

			if err := controller.StartManager(conf); err != nil {
				return fmt.Errorf("failed to start control loop: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Var(
		&gatewayCtlrName,
		gatewayCtlrNameFlag,
		fmt.Sprintf(gatewayCtlrNameUsageFmt, domain),
	)
	utilruntime.Must(cmd.MarkFlagRequired(gatewayCtlrNameFlag))

	cmd.Flags().Var(
		&gatewayClassName,
		gatewayClassFlag,
		gatewayClassNameUsage,
	)
	utilruntime.Must(cmd.MarkFlagRequired(gatewayClassFlag))

	cmd.Flags().VarP(
		&configName,
		configFlag,
		"c",
		`The name of the NginxGateway resource to be used for this controller's dynamic configuration.`+
			` Lives in the same Namespace as the controller.`,
	)

	cmd.Flags().Var(
		&serviceName,
		serviceFlag,
		`The name of the Service that fronts this NGINX Gateway Fabric Pod.`+
			` Lives in the same Namespace as the controller.`,
	)

	cmd.Flags().Var(
		&agentTLSSecretName,
		agentTLSSecretFlag,
		`The name of the base Secret containing TLS CA, certificate, and key for the NGINX Agent to securely `+
			`communicate with the NGINX Gateway Fabric control plane. Must exist in the same namespace that the `+
			`NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).`,
	)

	cmd.Flags().Var(
		&nginxOneConsoleDataplaneKeySecretName,
		nginxOneDataplaneKeySecretFlag,
		`The name of the Secret containing the NGINX One Console's dataplane key. Must exist in the same namespace that `+
			`the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).`,
	)

	cmd.Flags().Var(
		&nginxOneConsoleTelemetryEndpointHost,
		nginxOneTelemetryEndpointHostFlag,
		`The host of the NGINX One Console's telemetry endpoint.`,
	)

	cmd.Flags().Var(
		&nginxOneConsoleTelemetryEndpointPort,
		nginxOneTelemetryEndpointPortFlag,
		`The port of the NGINX One Console's telemetry endpoint.`,
	)

	cmd.Flags().BoolVar(
		&nginxOneConsoleTLSSkipVerify,
		nginxOneTLSSkipVerifyFlag,
		false,
		"Disable client verification of the NGINX One Console's telemetry endpoint server certificate.",
	)

	cmd.Flags().BoolVar(
		&disableMetrics,
		metricsDisableFlag,
		false,
		"Disable exposing metrics in the Prometheus format.",
	)

	cmd.Flags().Var(
		&metricsListenPort,
		metricsPortFlag,
		"Set the port where the metrics are exposed. Format: [1024 - 65535]",
	)

	cmd.Flags().BoolVar(
		&metricsSecure,
		metricsSecureFlag,
		false,
		"Enable serving metrics via https. By default metrics are served via http."+
			" Please note that this endpoint will be secured with a self-signed certificate.",
	)

	cmd.Flags().BoolVar(
		&disableHealth,
		healthDisableFlag,
		false,
		"Disable running the health probe server.",
	)

	cmd.Flags().Var(
		&healthListenPort,
		healthPortFlag,
		"Set the port where the health probe server is exposed. Format: [1024 - 65535]",
	)

	cmd.Flags().BoolVar(
		&disableLeaderElection,
		leaderElectionDisableFlag,
		false,
		"Disable leader election. Leader election is used to avoid multiple replicas of the NGINX Gateway Fabric"+
			" reporting the status of the Gateway API resources. If disabled, "+
			"all replicas of NGINX Gateway Fabric will update the statuses of the Gateway API resources.",
	)

	cmd.Flags().Var(
		&leaderElectionLockName,
		leaderElectionLockNameFlag,
		"The name of the leader election lock. "+
			"A Lease object with this name will be created in the same Namespace as the controller.",
	)

	cmd.Flags().BoolVar(
		&disableProductTelemetry,
		productTelemetryDisableFlag,
		false,
		"Disable the collection of product telemetry.",
	)

	cmd.Flags().BoolVar(
		&plus,
		plusFlag,
		false,
		"Use NGINX Plus",
	)

	cmd.Flags().BoolVar(
		&gwExperimentalFeatures,
		gwAPIExperimentalFlag,
		false,
		"Enable the experimental features of Gateway API which are supported by NGINX Gateway Fabric. "+
			"Requires the Gateway APIs installed from the experimental channel.",
	)

	cmd.Flags().BoolVar(
		&gwInferenceExtension,
		gwAPIInferenceExtensionFlag,
		false,
		"Enable Gateway API Inference Extension support. Allows for configuring InferencePools to route "+
			"traffic to AI workloads.",
	)

	addEPPConnectionFlags(cmd, &endpointPickerDisableTLS, &endpointPickerTLSSkipVerify)

	cmd.Flags().Var(
		&nginxDockerSecrets,
		nginxDockerSecretFlag,
		"The name of the NGINX docker registry Secret(s). Must exist in the same namespace "+
			"that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).",
	)

	cmd.Flags().Var(
		&usageReportParams.SecretName,
		usageReportSecretFlag,
		"The name of the Secret containing the JWT for NGINX Plus usage reporting. Must exist in the same namespace "+
			"that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).",
	)

	cmd.Flags().Var(
		&usageReportParams.Endpoint,
		usageReportEndpointFlag,
		"The endpoint of the NGINX Plus usage reporting server.",
	)

	cmd.Flags().Var(
		&usageReportParams.Resolver,
		usageReportResolverFlag,
		"The nameserver used to resolve the NGINX Plus usage reporting endpoint. Used with NGINX Instance Manager.",
	)

	cmd.Flags().BoolVar(
		&usageReportParams.SkipVerify,
		usageReportSkipVerifyFlag,
		false,
		"Disable client verification of the NGINX Plus usage reporting server certificate.",
	)

	cmd.Flags().Var(
		&usageReportParams.ClientSSLSecretName,
		usageReportClientSSLSecretFlag,
		"The name of the Secret containing the client certificate and key for authenticating with NGINX Instance Manager. "+
			"Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in "+
			"(default namespace: nginx-gateway).",
	)

	cmd.Flags().Var(
		&usageReportParams.CASecretName,
		usageReportCASecretFlag,
		"The name of the Secret containing the NGINX Instance Manager CA certificate. "+
			"Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in "+
			"(default namespace: nginx-gateway).",
	)

	cmd.Flags().BoolVar(
		&usageReportParams.EnforceInitialReport,
		usageReportEnforceInitialReportFlag,
		true,
		"Enable enforcement of the initial NGINX Plus licensing report. If set to false, the initial report is not enforced.",
	)

	cmd.Flags().BoolVar(
		&snippetsFilters,
		snippetsFiltersFlag,
		false,
		"Enable SnippetsFilters feature. SnippetsFilters allow inserting NGINX configuration "+
			"into the generated NGINX config for HTTPRoute and GRPCRoute resources.",
	)
	_ = cmd.Flags().MarkDeprecated(snippetsFiltersFlag, "This flag is deprecated in favor of --snippets, "+
		"which enables both SnippetsFilters and SnippetsPolicies.")

	cmd.Flags().BoolVar(
		&snippets,
		snippetsFlag,
		false,
		"Enable Snippets feature through SnippetsFilter and SnippetsPolicy APIs. SnippetsFilters allow inserting "+
			"NGINX configuration into the generated NGINX config for HTTPRoute and GRPCRoute resources. SnippetsPolicies "+
			"allow inserting NGINX configuration into the generated NGINX config for Gateway resources.",
	)

	cmd.Flags().Var(
		&nginxSCCName,
		nginxSCCFlag,
		`The name of the SecurityContextConstraints to be used with the NGINX data plane Pods.`+
			` Only applicable in OpenShift.`,
	)

	cmd.Flags().Var(
		&watchNamespaces,
		watchNamespacesFlag,
		`Comma-separated list of namespaces to watch for resources. If not set, all namespaces are watched. `+
			`The controller's own namespace is always watched.`,
	)

	return cmd
}

func buildUsageReportConfig(params usageReportParams) (config.UsageReportConfig, error) {
	if params.SecretName.value == "" {
		return config.UsageReportConfig{}, errors.New("usage-report-secret is required when using NGINX Plus")
	}

	return config.UsageReportConfig{
		SecretName:           params.SecretName.value,
		ClientSSLSecretName:  params.ClientSSLSecretName.value,
		CASecretName:         params.CASecretName.value,
		Endpoint:             params.Endpoint.value,
		Resolver:             params.Resolver.value,
		SkipVerify:           params.SkipVerify,
		EnforceInitialReport: params.EnforceInitialReport,
	}, nil
}

func createGenerateCertsCommand() *cobra.Command {
	// flag names
	const (
		serverTLSSecretFlag = "server-tls-secret" //nolint:gosec // not credentials
		agentTLSSecretFlag  = "agent-tls-secret"
		serviceFlag         = "service"
		clusterDomainFlag   = "cluster-domain"
		overwriteFlag       = "overwrite"
	)

	// flag values
	var (
		serverTLSSecretName = stringValidatingValue{
			validator: validateResourceName,
			value:     serverTLSSecret,
		}
		agentTLSSecretName = stringValidatingValue{
			validator: validateResourceName,
			value:     agentTLSSecret,
		}
		serviceName = stringValidatingValue{
			validator: validateResourceName,
		}
		clusterDomain = stringValidatingValue{
			validator: validateQualifiedName,
			value:     defaultDomain,
		}
		overwrite bool
	)

	cmd := &cobra.Command{
		Use:   "generate-certs",
		Short: "Generate self-signed certificates for securing control plane to data plane communication",
		RunE: func(cmd *cobra.Command, _ []string) error {
			namespace, err := getValueFromEnv("POD_NAMESPACE")
			if err != nil {
				return fmt.Errorf("POD_NAMESPACE must be specified in the ENV")
			}

			certConfig, err := generateCertificates(serviceName.value, namespace, clusterDomain.value)
			if err != nil {
				return fmt.Errorf("error generating certificates: %w", err)
			}

			k8sClient, err := client.New(k8sConfig.GetConfigOrDie(), client.Options{})
			if err != nil {
				return fmt.Errorf("error creating k8s client: %w", err)
			}

			if err := createSecrets(
				cmd.Context(),
				k8sClient,
				certConfig,
				serverTLSSecretName.value,
				agentTLSSecretName.value,
				namespace,
				overwrite,
			); err != nil {
				return fmt.Errorf("error creating secrets: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().Var(
		&serverTLSSecretName,
		serverTLSSecretFlag,
		`The name of the Secret containing TLS CA, certificate, and key for the NGINX Gateway Fabric control plane `+
			`to securely communicate with the NGINX Agent. Must exist in the same namespace that the `+
			`NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).`,
	)

	cmd.Flags().Var(
		&agentTLSSecretName,
		agentTLSSecretFlag,
		`The name of the base Secret containing TLS CA, certificate, and key for the NGINX Agent to securely `+
			`communicate with the NGINX Gateway Fabric control plane. Must exist in the same namespace that the `+
			`NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).`,
	)

	cmd.Flags().Var(
		&serviceName,
		serviceFlag,
		`The name of the Service that fronts the NGINX Gateway Fabric Pod.`+
			` Lives in the same Namespace as the controller.`,
	)

	cmd.Flags().Var(
		&clusterDomain,
		clusterDomainFlag,
		`The DNS domain of your Kubernetes cluster.`,
	)

	cmd.Flags().BoolVar(
		&overwrite,
		overwriteFlag,
		false,
		"Overwrite existing certificates.",
	)

	return cmd
}

func createInitializeCommand() *cobra.Command {
	// flag names
	const srcFlag = "source"
	const destFlag = "destination"

	// flag values
	var srcFiles []string
	var destDirs []string
	var plus bool

	cmd := &cobra.Command{
		Use:   "initialize",
		Short: "Write initial configuration files",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := validateCopyArgs(srcFiles, destDirs); err != nil {
				return err
			}

			podUID, err := getValueFromEnv("POD_UID")
			if err != nil {
				return fmt.Errorf("could not get pod UID: %w", err)
			}

			clusterUID, err := getValueFromEnv("CLUSTER_UID")
			if err != nil {
				return fmt.Errorf("could not get cluster UID: %w", err)
			}

			logger := ctlrZap.New()
			klog.SetLogger(logger)
			logger.Info(
				"Starting init container",
				"source filenames to copy", srcFiles,
				"destination directories", destDirs,
				"nginx-plus",
				plus,
			)
			log.SetLogger(logger)

			files := make([]fileToCopy, 0, len(srcFiles))
			for i, src := range srcFiles {
				files = append(files, fileToCopy{
					destDirName: destDirs[i],
					srcFileName: src,
				})
			}

			return initialize(initializeConfig{
				fileManager:   file.NewStdLibOSFileManager(),
				fileGenerator: ngxConfig.NewGeneratorImpl(plus, nil, logger.WithName("generator")),
				logger:        logger,
				podUID:        podUID,
				clusterUID:    clusterUID,
				plus:          plus,
				copy:          files,
			})
		},
	}

	cmd.Flags().StringSliceVar(
		&srcFiles,
		srcFlag,
		[]string{},
		"The source files to be copied",
	)

	cmd.Flags().StringSliceVar(
		&destDirs,
		destFlag,
		[]string{},
		"The destination directories for the source files at the same array index to be copied to",
	)

	cmd.Flags().BoolVar(
		&plus,
		plusFlag,
		false,
		"Use NGINX Plus",
	)

	cmd.MarkFlagsRequiredTogether(srcFlag, destFlag)

	return cmd
}

// FIXME(pleshakov): Remove this command once NGF min supported Kubernetes version supports sleep action in
// preStop hook.
// See https://github.com/kubernetes/enhancements/tree/4ec371d92dcd4f56a2ab18c8ba20bb85d8d20efe/keps/sig-node/3960-pod-lifecycle-sleep-action
//
//nolint:lll
func createSleepCommand() *cobra.Command {
	// flag names
	const durationFlag = "duration"
	// flag values
	var duration time.Duration

	cmd := &cobra.Command{
		Use:   "sleep",
		Short: "Sleep for specified duration and exit",
		Run: func(_ *cobra.Command, _ []string) {
			// It is expected that this command is run from lifecycle hook.
			// Because logs from hooks are not visible in the container logs, we don't log here at all.
			time.Sleep(duration)
		},
	}

	cmd.Flags().DurationVar(
		&duration,
		durationFlag,
		30*time.Second,
		"Set the duration of sleep. Must be parsable by https://pkg.go.dev/time#ParseDuration",
	)

	return cmd
}

func createEndpointPickerCommand() *cobra.Command {
	var endpointPickerDisableTLS bool
	endpointPickerTLSSkipVerify := true
	cmd := &cobra.Command{
		Use:   "endpoint-picker",
		Short: "Shim server for communication between NGINX and the Gateway API Inference Extension Endpoint Picker",
		RunE: func(_ *cobra.Command, _ []string) error {
			logger := ctlrZap.New().WithName("endpoint-picker-shim")
			handler := createEndpointPickerHandler(
				realExtProcClientFactory(endpointPickerDisableTLS, endpointPickerTLSSkipVerify),
				logger,
			)
			return endpointPickerServer(handler)
		},
	}

	addEPPConnectionFlags(cmd, &endpointPickerDisableTLS, &endpointPickerTLSSkipVerify)

	return cmd
}

func addEPPConnectionFlags(cmd *cobra.Command, disableTLS, tlsSkipVerify *bool) {
	cmd.Flags().BoolVar(
		disableTLS,
		endpointPickerDisableTLSFlag,
		false,
		"Disables TLS when connecting to the EndpointPicker. "+
			"Set to true only for development/testing or when using a service mesh for encryption.",
	)

	cmd.Flags().BoolVar(
		tlsSkipVerify,
		endpointPickerTLSSkipVerifyFlag,
		true,
		"Disables server certificate verification when connecting to the EndpointPicker, if TLS is enabled. "+
			"REQUIRED: Must be true until Gateway API Inference Extension EndpointPicker supports mounting certificates.",
	)
}

func parseFlags(flags *pflag.FlagSet) ([]string, []string) {
	var flagKeys, flagValues []string

	flags.VisitAll(
		func(flag *pflag.Flag) {
			flagKeys = append(flagKeys, flag.Name)

			if flag.Value.Type() == "bool" {
				flagValues = append(flagValues, flag.Value.String())
			} else {
				val := "user-defined"
				if flag.Value.String() == flag.DefValue {
					val = "default"
				}

				flagValues = append(flagValues, val)
			}
		},
	)

	return flagKeys, flagValues
}

func getBuildInfo() (commitHash string, commitTime string, dirtyBuild string) {
	commitHash = "unknown"
	commitTime = "unknown"
	dirtyBuild = "unknown"

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return commitHash, commitTime, dirtyBuild
	}
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			commitHash = kv.Value
		case "vcs.time":
			commitTime = kv.Value
		case "vcs.modified":
			dirtyBuild = kv.Value
		}
	}

	return commitHash, commitTime, dirtyBuild
}

func createGatewayPodConfig(version, svcName string) (config.GatewayPodConfig, error) {
	podUID, err := getValueFromEnv("POD_UID")
	if err != nil {
		return config.GatewayPodConfig{}, err
	}

	ns, err := getValueFromEnv("POD_NAMESPACE")
	if err != nil {
		return config.GatewayPodConfig{}, err
	}

	name, err := getValueFromEnv("POD_NAME")
	if err != nil {
		return config.GatewayPodConfig{}, err
	}

	instance, err := getValueFromEnv("INSTANCE_NAME")
	if err != nil {
		return config.GatewayPodConfig{}, err
	}

	image, err := getValueFromEnv("IMAGE_NAME")
	if err != nil {
		return config.GatewayPodConfig{}, err
	}

	c := config.GatewayPodConfig{
		ServiceName:  svcName,
		Namespace:    ns,
		Name:         name,
		UID:          podUID,
		InstanceName: instance,
		Version:      version,
		Image:        image,
	}

	return c, nil
}

func getValueFromEnv(key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("environment variable %s not set", key)
	}

	return val, nil
}
