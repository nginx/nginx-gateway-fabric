package telemetry

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"

	tel "github.com/nginx/telemetry-exporter/pkg/telemetry"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sversion "k8s.io/apimachinery/pkg/util/version"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/config"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/graph"
)

//counterfeiter:generate . GraphGetter

// GraphGetter gets the latest Graph.
type GraphGetter interface {
	GetLatestGraph() *graph.Graph
}

//counterfeiter:generate . ConfigurationGetter

// ConfigurationGetter gets the latest Configuration.
type ConfigurationGetter interface {
	GetLatestConfiguration() *dataplane.Configuration
}

// Data is telemetry data.
//
//go:generate go run -tags generator github.com/nginx/telemetry-exporter/cmd/generator -type=Data -scheme -scheme-protocol=NGFProductTelemetry -scheme-df-datatype=ngf-product-telemetry
type Data struct {
	// ImageSource tells whether the image was built by GitHub or locally (values are 'gha', 'local', or 'unknown')
	ImageSource string
	tel.Data    // embedding is required by the generator.
	// FlagNames contains the command-line flag names.
	FlagNames []string
	// FlagValues contains the values of the command-line flags, where each value corresponds to the flag from FlagNames
	// at the same index.
	// Each value is either 'true' or 'false' for boolean flags and 'default' or 'user-defined' for non-boolean flags.
	FlagValues []string
	// SnippetsFiltersDirectives contains the directive-context strings of all applied SnippetsFilters.
	// Both lists are ordered first by count, then by lexicographical order of the context string,
	// then lastly by directive string.
	SnippetsFiltersDirectives []string
	// SnippetsFiltersDirectivesCount contains the count of the directive-context strings, where each count
	// corresponds to the string from SnippetsFiltersDirectives at the same index.
	// Both lists are ordered first by count, then by lexicographical order of the context string,
	// then lastly by directive string.
	SnippetsFiltersDirectivesCount []int64
	NGFResourceCounts              // embedding is required by the generator.
	// NGFReplicaCount is the number of replicas of the NGF Pod.
	NGFReplicaCount int64
}

// NGFResourceCounts stores the counts of all relevant resources that NGF processes and generates configuration from.
//
//go:generate go run -tags generator github.com/nginx/telemetry-exporter/cmd/generator -type=NGFResourceCounts
type NGFResourceCounts struct {
	// GatewayCount is the number of relevant Gateways.
	GatewayCount int64
	// GatewayClassCount is the number of relevant GatewayClasses.
	GatewayClassCount int64
	// HTTPRouteCount is the number of relevant HTTPRoutes.
	HTTPRouteCount int64
	// TLSRouteCount is the number of relevant TLSRoutes.
	TLSRouteCount int64
	// SecretCount is the number of relevant Secrets.
	SecretCount int64
	// ServiceCount is the number of relevant Services.
	ServiceCount int64
	// EndpointCount include the total count of Endpoints(IP:port) across all referenced services.
	EndpointCount int64
	// GRPCRouteCount is the number of relevant GRPCRoutes.
	GRPCRouteCount int64
	// BackendTLSPolicyCount is the number of relevant BackendTLSPolicies.
	BackendTLSPolicyCount int64
	// GatewayAttachedClientSettingsPolicyCount is the number of relevant ClientSettingsPolicies
	// attached at the Gateway level.
	GatewayAttachedClientSettingsPolicyCount int64
	// RouteAttachedClientSettingsPolicyCount is the number of relevant ClientSettingsPolicies attached at the Route level.
	RouteAttachedClientSettingsPolicyCount int64
	// ObservabilityPolicyCount is the number of relevant ObservabilityPolicies.
	ObservabilityPolicyCount int64
	// NginxProxyCount is the number of NginxProxies.
	NginxProxyCount int64
	// SnippetsFilterCount is the number of SnippetsFilters.
	SnippetsFilterCount int64
	// UpstreamSettingsPolicyCount is the number of UpstreamSettingsPolicies.
	UpstreamSettingsPolicyCount int64
}

// DataCollectorConfig holds configuration parameters for DataCollectorImpl.
type DataCollectorConfig struct {
	// K8sClientReader is a Kubernetes API client Reader.
	K8sClientReader client.Reader
	// GraphGetter allows us to get the Graph.
	GraphGetter GraphGetter
	// ConfigurationGetter allows us to get the Configuration.
	ConfigurationGetter ConfigurationGetter
	// Version is the NGF version.
	Version string
	// PodNSName is the NamespacedName of the NGF Pod.
	PodNSName types.NamespacedName
	// ImageSource is the source of the NGF image.
	ImageSource string
	// Flags contains the command-line NGF flag keys and values.
	Flags config.Flags
}

// DataCollectorImpl is am implementation of DataCollector.
type DataCollectorImpl struct {
	cfg DataCollectorConfig
}

// NewDataCollectorImpl creates a new DataCollectorImpl for a telemetry Job.
func NewDataCollectorImpl(
	cfg DataCollectorConfig,
) *DataCollectorImpl {
	return &DataCollectorImpl{
		cfg: cfg,
	}
}

// Collect collects and returns telemetry Data.
func (c DataCollectorImpl) Collect(ctx context.Context) (Data, error) {
	g := c.cfg.GraphGetter.GetLatestGraph()
	if g == nil {
		return Data{}, errors.New("failed to collect telemetry data: latest graph cannot be nil")
	}

	clusterInfo, err := CollectClusterInformation(ctx, c.cfg.K8sClientReader)
	if err != nil {
		return Data{}, fmt.Errorf("failed to collect cluster information: %w", err)
	}

	graphResourceCount, err := collectGraphResourceCount(g, c.cfg.ConfigurationGetter)
	if err != nil {
		return Data{}, fmt.Errorf("failed to collect NGF resource counts: %w", err)
	}

	replicaSet, err := getPodReplicaSet(ctx, c.cfg.K8sClientReader, c.cfg.PodNSName)
	if err != nil {
		return Data{}, fmt.Errorf("failed to get replica set for pod %v: %w", c.cfg.PodNSName, err)
	}

	replicaCount, err := getReplicas(replicaSet)
	if err != nil {
		return Data{}, fmt.Errorf("failed to collect NGF replica count: %w", err)
	}

	deploymentID, err := getDeploymentID(replicaSet)
	if err != nil {
		return Data{}, fmt.Errorf("failed to get NGF deploymentID: %w", err)
	}

	snippetsFiltersDirectives, snippetsFiltersDirectivesCount := collectSnippetsFilterDirectives(g)

	data := Data{
		Data: tel.Data{
			ProjectName:         "NGF",
			ProjectVersion:      c.cfg.Version,
			ProjectArchitecture: runtime.GOARCH,
			ClusterID:           clusterInfo.ClusterID,
			ClusterVersion:      clusterInfo.Version,
			ClusterPlatform:     clusterInfo.Platform,
			InstallationID:      deploymentID,
			ClusterNodeCount:    int64(clusterInfo.NodeCount),
		},
		NGFResourceCounts:              graphResourceCount,
		ImageSource:                    c.cfg.ImageSource,
		FlagNames:                      c.cfg.Flags.Names,
		FlagValues:                     c.cfg.Flags.Values,
		NGFReplicaCount:                int64(replicaCount),
		SnippetsFiltersDirectives:      snippetsFiltersDirectives,
		SnippetsFiltersDirectivesCount: snippetsFiltersDirectivesCount,
	}

	return data, nil
}

func collectGraphResourceCount(
	g *graph.Graph,
	configurationGetter ConfigurationGetter,
) (NGFResourceCounts, error) {
	ngfResourceCounts := NGFResourceCounts{}
	cfg := configurationGetter.GetLatestConfiguration()

	if cfg == nil {
		return ngfResourceCounts, errors.New("latest configuration cannot be nil")
	}

	ngfResourceCounts.GatewayClassCount = int64(len(g.IgnoredGatewayClasses))
	if g.GatewayClass != nil {
		ngfResourceCounts.GatewayClassCount++
	}

	ngfResourceCounts.GatewayCount = int64(len(g.IgnoredGateways))
	if g.Gateway != nil {
		ngfResourceCounts.GatewayCount++
	}

	routeCounts := computeRouteCount(g.Routes, g.L4Routes)
	ngfResourceCounts.HTTPRouteCount = routeCounts.HTTPRouteCount
	ngfResourceCounts.GRPCRouteCount = routeCounts.GRPCRouteCount
	ngfResourceCounts.TLSRouteCount = routeCounts.TLSRouteCount

	ngfResourceCounts.SecretCount = int64(len(g.ReferencedSecrets))
	ngfResourceCounts.ServiceCount = int64(len(g.ReferencedServices))

	for _, upstream := range cfg.Upstreams {
		if upstream.ErrorMsg == "" {
			ngfResourceCounts.EndpointCount += int64(len(upstream.Endpoints))
		}
	}

	ngfResourceCounts.BackendTLSPolicyCount = int64(len(g.BackendTLSPolicies))

	for policyKey, policy := range g.NGFPolicies {
		switch policyKey.GVK.Kind {
		case kinds.ClientSettingsPolicy:
			if len(policy.TargetRefs) == 0 {
				continue
			}

			if policy.TargetRefs[0].Kind == kinds.Gateway {
				ngfResourceCounts.GatewayAttachedClientSettingsPolicyCount++
			} else {
				ngfResourceCounts.RouteAttachedClientSettingsPolicyCount++
			}
		case kinds.ObservabilityPolicy:
			ngfResourceCounts.ObservabilityPolicyCount++
		case kinds.UpstreamSettingsPolicy:
			ngfResourceCounts.UpstreamSettingsPolicyCount++
		}
	}

	if g.NginxProxy != nil {
		ngfResourceCounts.NginxProxyCount = 1
	}

	ngfResourceCounts.SnippetsFilterCount = int64(len(g.SnippetsFilters))

	return ngfResourceCounts, nil
}

type RouteCounts struct {
	HTTPRouteCount int64
	GRPCRouteCount int64
	TLSRouteCount  int64
}

func computeRouteCount(
	routes map[graph.RouteKey]*graph.L7Route,
	l4routes map[graph.L4RouteKey]*graph.L4Route,
) RouteCounts {
	httpRouteCount := int64(0)
	grpcRouteCount := int64(0)

	for _, r := range routes {
		if r.RouteType == graph.RouteTypeHTTP {
			httpRouteCount++
		}
		if r.RouteType == graph.RouteTypeGRPC {
			grpcRouteCount++
		}
	}

	return RouteCounts{
		HTTPRouteCount: httpRouteCount,
		GRPCRouteCount: grpcRouteCount,
		TLSRouteCount:  int64(len(l4routes)),
	}
}

// getPodReplicaSet returns the replicaset for the provided Pod.
func getPodReplicaSet(
	ctx context.Context,
	k8sClient client.Reader,
	podNSName types.NamespacedName,
) (*appsv1.ReplicaSet, error) {
	var pod v1.Pod
	if err := k8sClient.Get(
		ctx,
		types.NamespacedName{Namespace: podNSName.Namespace, Name: podNSName.Name},
		&pod,
	); err != nil {
		return nil, fmt.Errorf("failed to get NGF Pod: %w", err)
	}

	podOwnerRefs := pod.GetOwnerReferences()
	if len(podOwnerRefs) != 1 {
		return nil, fmt.Errorf("expected one owner reference of the NGF Pod, got %d", len(podOwnerRefs))
	}

	if podOwnerRefs[0].Kind != "ReplicaSet" {
		return nil, fmt.Errorf("expected pod owner reference to be ReplicaSet, got %s", podOwnerRefs[0].Kind)
	}

	var replicaSet appsv1.ReplicaSet
	if err := k8sClient.Get(
		ctx,
		types.NamespacedName{Namespace: podNSName.Namespace, Name: podOwnerRefs[0].Name},
		&replicaSet,
	); err != nil {
		return nil, fmt.Errorf("failed to get NGF Pod's ReplicaSet: %w", err)
	}

	return &replicaSet, nil
}

func getReplicas(replicaSet *appsv1.ReplicaSet) (int, error) {
	if replicaSet.Spec.Replicas == nil {
		return 0, errors.New("replica set replicas was nil")
	}

	return int(*replicaSet.Spec.Replicas), nil
}

// getDeploymentID gets the deployment ID of the provided ReplicaSet.
func getDeploymentID(replicaSet *appsv1.ReplicaSet) (string, error) {
	replicaOwnerRefs := replicaSet.GetOwnerReferences()
	if len(replicaOwnerRefs) != 1 {
		return "", fmt.Errorf("expected one owner reference of the NGF ReplicaSet, got %d", len(replicaOwnerRefs))
	}

	if replicaOwnerRefs[0].Kind != "Deployment" {
		return "", fmt.Errorf("expected replicaSet owner reference to be Deployment, got %s", replicaOwnerRefs[0].Kind)
	}

	if replicaOwnerRefs[0].UID == "" {
		return "", fmt.Errorf("expected replicaSet owner reference to have a UID")
	}

	return string(replicaOwnerRefs[0].UID), nil
}

// collectClusterID gets the UID of the kube-system namespace.
func collectClusterID(ctx context.Context, k8sClient client.Reader) (string, error) {
	key := types.NamespacedName{
		Name: metav1.NamespaceSystem,
	}
	var kubeNamespace v1.Namespace
	if err := k8sClient.Get(ctx, key, &kubeNamespace); err != nil {
		return "", fmt.Errorf("failed to get kube-system namespace: %w", err)
	}
	return string(kubeNamespace.GetUID()), nil
}

type ClusterInformation struct {
	Platform  string
	Version   string
	ClusterID string
	NodeCount int
}

// CollectClusterInformation collects information about the cluster.
func CollectClusterInformation(ctx context.Context, k8sClient client.Reader) (ClusterInformation, error) {
	var clusterInfo ClusterInformation

	var nodes v1.NodeList
	if err := k8sClient.List(ctx, &nodes); err != nil {
		return ClusterInformation{}, fmt.Errorf("failed to get NodeList: %w", err)
	}

	nodeCount := len(nodes.Items)
	if nodeCount == 0 {
		return ClusterInformation{}, errors.New("failed to collect cluster information: NodeList length is zero")
	}
	clusterInfo.NodeCount = nodeCount

	node := nodes.Items[0]

	kubeletVersion := node.Status.NodeInfo.KubeletVersion
	version, err := k8sversion.ParseGeneric(kubeletVersion)
	if err != nil {
		clusterInfo.Version = "unknown"
	} else {
		clusterInfo.Version = version.String()
	}

	var namespaces v1.NamespaceList
	if err = k8sClient.List(ctx, &namespaces); err != nil {
		return ClusterInformation{}, fmt.Errorf("failed to collect cluster information: %w", err)
	}

	clusterInfo.Platform = getPlatform(node, namespaces)

	var clusterID string
	clusterID, err = collectClusterID(ctx, k8sClient)
	if err != nil {
		return ClusterInformation{}, fmt.Errorf("failed to collect cluster information: %w", err)
	}
	clusterInfo.ClusterID = clusterID

	return clusterInfo, nil
}

type sfDirectiveContext struct {
	directive string
	context   string
}

func collectSnippetsFilterDirectives(g *graph.Graph) ([]string, []int64) {
	directiveContextMap := make(map[sfDirectiveContext]int)

	for _, sf := range g.SnippetsFilters {
		if sf == nil {
			continue
		}

		for nginxContext, snippetValue := range sf.Snippets {
			var parsedContext string

			switch nginxContext {
			case ngfAPI.NginxContextMain:
				parsedContext = "main"
			case ngfAPI.NginxContextHTTP:
				parsedContext = "http"
			case ngfAPI.NginxContextHTTPServer:
				parsedContext = "server"
			case ngfAPI.NginxContextHTTPServerLocation:
				parsedContext = "location"
			default:
				parsedContext = "unknown"
			}

			directives := parseSnippetValueIntoDirectives(snippetValue)
			for _, directive := range directives {
				directiveContext := sfDirectiveContext{
					directive: directive,
					context:   parsedContext,
				}
				directiveContextMap[directiveContext]++
			}
		}
	}

	return parseDirectiveContextMapIntoLists(directiveContextMap)
}

func parseSnippetValueIntoDirectives(snippetValue string) []string {
	separatedDirectives := strings.Split(snippetValue, ";")
	directives := make([]string, 0, len(separatedDirectives))

	for _, directive := range separatedDirectives {
		// the strings.TrimSpace is needed in the case of multi-line NGINX Snippet values
		directive = strings.Split(strings.TrimSpace(directive), " ")[0]

		// splitting on the delimiting character can result in a directive being empty or a space/newline character,
		// so we check here to ensure it's not
		if directive != "" {
			directives = append(directives, directive)
		}
	}

	return directives
}

// parseDirectiveContextMapIntoLists returns two same-length lists where the elements at each corresponding index
// are paired together.
// The first list contains strings which are the NGINX directive and context of a Snippet joined with a hyphen.
// The second list contains ints which are the count of total same directive-context values of the first list.
// Both lists are ordered first by count, then by lexicographical order of the context string,
// then lastly by directive string.
func parseDirectiveContextMapIntoLists(directiveContextMap map[sfDirectiveContext]int) ([]string, []int64) {
	type sfDirectiveContextCount struct {
		directive, context string
		count              int64
	}

	kvPairs := make([]sfDirectiveContextCount, 0, len(directiveContextMap))

	for k, v := range directiveContextMap {
		kvPairs = append(kvPairs, sfDirectiveContextCount{k.directive, k.context, int64(v)})
	}

	sort.Slice(kvPairs, func(i, j int) bool {
		if kvPairs[i].count == kvPairs[j].count {
			if kvPairs[i].context == kvPairs[j].context {
				return kvPairs[i].directive < kvPairs[j].directive
			}
			return kvPairs[i].context < kvPairs[j].context
		}
		return kvPairs[i].count > kvPairs[j].count
	})

	directiveContextList := make([]string, len(kvPairs))
	countList := make([]int64, len(kvPairs))

	for i, pair := range kvPairs {
		directiveContextList[i] = pair.directive + "-" + pair.context
		countList[i] = pair.count
	}

	return directiveContextList, countList
}
