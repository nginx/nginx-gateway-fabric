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
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	ctlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctlrZap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/provisioner"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/licensing"
	ngxConfig "github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/nginx/file"
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
)

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

func createStaticModeCommand() *cobra.Command {
	// flag names
	const (
		gatewayFlag                    = "gateway"
		configFlag                     = "config"
		serviceFlag                    = "service"
		updateGCStatusFlag             = "update-gatewayclass-status"
		metricsDisableFlag             = "metrics-disable"
		metricsSecureFlag              = "metrics-secure-serving"
		metricsPortFlag                = "metrics-port"
		healthDisableFlag              = "health-disable"
		healthPortFlag                 = "health-port"
		leaderElectionDisableFlag      = "leader-election-disable"
		leaderElectionLockNameFlag     = "leader-election-lock-name"
		productTelemetryDisableFlag    = "product-telemetry-disable"
		gwAPIExperimentalFlag          = "gateway-api-experimental-features"
		usageReportSecretFlag          = "usage-report-secret"
		usageReportEndpointFlag        = "usage-report-endpoint"
		usageReportResolverFlag        = "usage-report-resolver"
		usageReportSkipVerifyFlag      = "usage-report-skip-verify"
		usageReportClientSSLSecretFlag = "usage-report-client-ssl-secret" //nolint:gosec // not credentials
		usageReportCASecretFlag        = "usage-report-ca-secret"         //nolint:gosec // not credentials
		snippetsFiltersFlag            = "snippets-filters"
	)

	// flag values
	var (
		gatewayCtlrName = stringValidatingValue{
			validator: validateGatewayControllerName,
		}

		gatewayClassName = stringValidatingValue{
			validator: validateResourceName,
		}

		updateGCStatus bool
		gateway        = namespacedNameValue{}
		configName     = stringValidatingValue{
			validator: validateResourceName,
		}
		serviceName = stringValidatingValue{
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

		disableProductTelemetry bool

		snippetsFilters bool

		plus                  bool
		usageReportSkipVerify bool
		usageReportSecretName = stringValidatingValue{
			validator: validateResourceName,
			value:     "nplus-license",
		}
		usageReportEndpoint = stringValidatingValue{
			validator: validateEndpointOptionalPort,
		}
		usageReportResolver = stringValidatingValue{
			validator: validateEndpointOptionalPort,
		}
		usageReportClientSSLSecretName = stringValidatingValue{
			validator: validateResourceName,
		}
		usageReportCASecretName = stringValidatingValue{
			validator: validateResourceName,
		}
	)

	cmd := &cobra.Command{
		Use:   "static-mode",
		Short: "Configure NGINX in the scope of a single Gateway resource",
		RunE: func(cmd *cobra.Command, _ []string) error {
			atom := zap.NewAtomicLevel()

			logger := ctlrZap.New(ctlrZap.Level(atom))
			klog.SetLogger(logger)

			commit, date, dirty := getBuildInfo()
			logger.Info(
				"Starting NGINX Gateway Fabric in static mode",
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

			var gwNsName *types.NamespacedName
			if cmd.Flags().Changed(gatewayFlag) {
				gwNsName = &gateway.value
			}

			var usageReportConfig config.UsageReportConfig
			if plus && usageReportSecretName.value == "" {
				return errors.New("usage-report-secret is required when using NGINX Plus")
			}

			if plus {
				usageReportConfig = config.UsageReportConfig{
					SecretName:          usageReportSecretName.value,
					ClientSSLSecretName: usageReportClientSSLSecretName.value,
					CASecretName:        usageReportCASecretName.value,
					Endpoint:            usageReportEndpoint.value,
					Resolver:            usageReportResolver.value,
					SkipVerify:          usageReportSkipVerify,
				}
			}

			flagKeys, flagValues := parseFlags(cmd.Flags())

			podConfig, err := createGatewayPodConfig(serviceName.value)
			if err != nil {
				return fmt.Errorf("error creating gateway pod config: %w", err)
			}

			conf := config.Config{
				GatewayCtlrName:          gatewayCtlrName.value,
				ConfigName:               configName.String(),
				Logger:                   logger,
				AtomicLevel:              atom,
				GatewayClassName:         gatewayClassName.value,
				GatewayNsName:            gwNsName,
				UpdateGatewayClassStatus: updateGCStatus,
				GatewayPodConfig:         podConfig,
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
				Version:              version,
				ExperimentalFeatures: gwExperimentalFeatures,
				ImageSource:          imageSource,
				Flags: config.Flags{
					Names:  flagKeys,
					Values: flagValues,
				},
				SnippetsFilters: snippetsFilters,
			}

			if err := static.StartManager(conf); err != nil {
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

	cmd.Flags().Var(
		&gateway,
		gatewayFlag,
		"The namespaced name of the Gateway resource to use. "+
			"Must be of the form: NAMESPACE/NAME. "+
			"If not specified, the control plane will process all Gateways for the configured GatewayClass. "+
			"However, among them, it will choose the oldest resource by creation timestamp. If the timestamps are "+
			"equal, it will choose the resource that appears first in alphabetical order by {namespace}/{name}.",
	)

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

	cmd.Flags().BoolVar(
		&updateGCStatus,
		updateGCStatusFlag,
		true,
		"Update the status of the GatewayClass resource.",
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

	cmd.Flags().Var(
		&usageReportSecretName,
		usageReportSecretFlag,
		"The name of the Secret containing the JWT for NGINX Plus usage reporting. Must exist in the same namespace "+
			"that the NGINX Gateway Fabric control plane is running in (default namespace: nginx-gateway).",
	)

	cmd.Flags().Var(
		&usageReportEndpoint,
		usageReportEndpointFlag,
		"The endpoint of the NGINX Plus usage reporting server.",
	)

	cmd.Flags().Var(
		&usageReportResolver,
		usageReportResolverFlag,
		"The nameserver used to resolve the NGINX Plus usage reporting endpoint. Used with NGINX Instance Manager.",
	)

	cmd.Flags().BoolVar(
		&usageReportSkipVerify,
		usageReportSkipVerifyFlag,
		false,
		"Disable client verification of the NGINX Plus usage reporting server certificate.",
	)

	cmd.Flags().Var(
		&usageReportClientSSLSecretName,
		usageReportClientSSLSecretFlag,
		"The name of the Secret containing the client certificate and key for authenticating with NGINX Instance Manager. "+
			"Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in "+
			"(default namespace: nginx-gateway).",
	)

	cmd.Flags().Var(
		&usageReportCASecretName,
		usageReportCASecretFlag,
		"The name of the Secret containing the NGINX Instance Manager CA certificate. "+
			"Must exist in the same namespace that the NGINX Gateway Fabric control plane is running in "+
			"(default namespace: nginx-gateway).",
	)

	cmd.Flags().BoolVar(
		&snippetsFilters,
		snippetsFiltersFlag,
		false,
		"Enable SnippetsFilters feature. SnippetsFilters allow inserting NGINX configuration into the "+
			"generated NGINX config for HTTPRoute and GRPCRoute resources.",
	)

	return cmd
}

func createProvisionerModeCommand() *cobra.Command {
	var (
		gatewayCtlrName = stringValidatingValue{
			validator: validateGatewayControllerName,
		}
		gatewayClassName = stringValidatingValue{
			validator: validateResourceName,
		}
	)

	cmd := &cobra.Command{
		Use:    "provisioner-mode",
		Short:  "Provision a static-mode NGINX Gateway Fabric Deployment per Gateway resource",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			logger := ctlrZap.New()
			commit, date, dirty := getBuildInfo()
			logger.Info(
				"Starting NGINX Gateway Fabric Provisioner",
				"version", version,
				"commit", commit,
				"date", date,
				"dirty", dirty,
			)

			return provisioner.StartManager(provisioner.Config{
				Logger:           logger,
				GatewayClassName: gatewayClassName.value,
				GatewayCtlrName:  gatewayCtlrName.value,
			})
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

func createInitializeCommand() *cobra.Command {
	// flag names
	const srcFlag = "source"
	const destFlag = "destination"

	// flag values
	var srcFiles []string
	var dest string
	var plus bool

	cmd := &cobra.Command{
		Use:   "initialize",
		Short: "Write initial configuration files",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := validateCopyArgs(srcFiles, dest); err != nil {
				return err
			}

			podUID, err := getValueFromEnv("POD_UID")
			if err != nil {
				return fmt.Errorf("could not get pod UID: %w", err)
			}

			clusterCfg := ctlr.GetConfigOrDie()
			k8sReader, err := client.New(clusterCfg, client.Options{})
			if err != nil {
				return fmt.Errorf("unable to initialize k8s client: %w", err)
			}

			logger := ctlrZap.New()
			klog.SetLogger(logger)
			logger.Info(
				"Starting init container",
				"source filenames to copy", srcFiles,
				"destination directory", dest,
				"nginx-plus",
				plus,
			)
			log.SetLogger(logger)

			dcc := licensing.NewDeploymentContextCollector(licensing.DeploymentContextCollectorConfig{
				K8sClientReader: k8sReader,
				PodUID:          podUID,
				Logger:          logger.WithName("deployCtxCollector"),
			})

			return initialize(initializeConfig{
				fileManager:   file.NewStdLibOSFileManager(),
				fileGenerator: ngxConfig.NewGeneratorImpl(plus, nil, logger.WithName("generator")),
				logger:        logger,
				plus:          plus,
				collector:     dcc,
				copy: copyFiles{
					srcFileNames: srcFiles,
					destDirName:  dest,
				},
			})
		},
	}

	cmd.Flags().StringSliceVar(
		&srcFiles,
		srcFlag,
		[]string{},
		"The source files to be copied",
	)

	cmd.Flags().StringVar(
		&dest,
		destFlag,
		"",
		"The destination directory for the source files to be copied to",
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
		return
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

	return
}

func createGatewayPodConfig(svcName string) (config.GatewayPodConfig, error) {
	podIP, err := getValueFromEnv("POD_IP")
	if err != nil {
		return config.GatewayPodConfig{}, err
	}

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

	c := config.GatewayPodConfig{
		PodIP:       podIP,
		ServiceName: svcName,
		Namespace:   ns,
		Name:        name,
		UID:         podUID,
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
