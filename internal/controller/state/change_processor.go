package state

import (
	"context"
	"maps"
	"sync"

	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	inference "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	v1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	"sigs.k8s.io/gateway-api/pkg/consts"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
	ngftypes "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch"
)

//go:generate go tool counterfeiter -generate

//counterfeiter:generate . ChangeProcessor

// ChangeProcessor processes the changes to resources and produces a graph-like representation
// of the Gateway configuration. It only supports one GatewayClass resource.
type ChangeProcessor interface {
	// CaptureUpsertChange captures an upsert change to a resource.
	// It panics if the resource is of unsupported type or if the passed Gateway is different from the one this
	// ChangeProcessor was created for.
	CaptureUpsertChange(obj client.Object)
	// CaptureDeleteChange captures a delete change to a resource.
	// The method panics if the resource is of unsupported type or if the passed Gateway is different from the one
	// this ChangeProcessor was created for.
	CaptureDeleteChange(resourceType ngftypes.ObjectType, nsname types.NamespacedName)
	// Process produces a graph-like representation of GatewayAPI resources.
	// If no changes were captured, the graph will be empty.
	Process(ctx context.Context) (graphCfg *graph.Graph)
	// GetLatestGraph returns the latest Graph.
	GetLatestGraph() *graph.Graph
	// ForceRebuild forces the next Process() call to perform a full graph rebuild,
	// without modifying the cluster state. Used when an external event (e.g. a WAF bundle
	// becoming available) must trigger a rebuild without an accompanying resource change.
	ForceRebuild()
}

// ChangeProcessorConfig holds configuration parameters for ChangeProcessorImpl.
type ChangeProcessorConfig struct {
	// Validators validate resources according to data-plane specific rules.
	Validators validation.Validators
	// EventRecorder records events for Kubernetes resources.
	EventRecorder events.EventRecorder
	// WAFFetcher fetches WAF policy bundles from HTTP/HTTPS URLs.
	WAFFetcher fetch.Fetcher
	// MustExtractGVK is a function that extracts schema.GroupVersionKind from a client.Object.
	MustExtractGVK kinds.MustExtractGVK
	// PlusSecrets is a list of secret files used for NGINX Plus reporting (JWT, client SSL, CA).
	PlusSecrets map[types.NamespacedName][]graph.PlusSecretFile
	// DiscoveredCRDs is a map of discovered CRDs in the cluster,
	// where the key is the CRD name and the value indicates if the CRD exists.
	DiscoveredCRDs map[string]bool
	// PolledWAFBundles returns the latest bundles fetched by WAF pollers.
	// These take precedence over graph-cached bundles during stale-bundle fallback,
	// preventing a graph rebuild from overwriting newer polled data with older cached data.
	// May be nil if WAF polling is not enabled.
	PolledWAFBundles func() map[graph.WAFBundleKey]*graph.WAFBundleData
	// Logger is the logger for this Change Processor.
	Logger logr.Logger
	// GatewayCtlrName is the name of the Gateway controller.
	GatewayCtlrName string
	// GatewayClassName is the name of the GatewayClass resource.
	GatewayClassName string
	// FeatureFlags holds the feature flags for building the Graph.
	FeatureFlags graph.FeatureFlags
	// Snippets indicates if Snippets are enabled. This will enable both SnippetsFilter and SnippetsPolicy APIs.
	Snippets bool
}

// ChangeProcessorImpl is an implementation of ChangeProcessor.
type ChangeProcessorImpl struct {
	latestGraph *graph.Graph

	// clusterState holds the current state of the cluster
	clusterState graph.ClusterState
	// updater acts upon the cluster state.
	updater Updater
	// getAndResetClusterStateChanged tells if and how the cluster state has changed.
	getAndResetClusterStateChanged func() bool
	// forceClusterStateRebuild forces the changed flag to true without modifying cluster state.
	forceClusterStateRebuild func()

	cfg  ChangeProcessorConfig
	lock sync.Mutex
}

// NewChangeProcessorImpl creates a new ChangeProcessorImpl for the Gateway resource with the configured namespace name.
func NewChangeProcessorImpl(cfg ChangeProcessorConfig) *ChangeProcessorImpl {
	clusterStore := graph.ClusterState{
		GatewayClasses:        make(map[types.NamespacedName]*v1.GatewayClass),
		Gateways:              make(map[types.NamespacedName]*v1.Gateway),
		HTTPRoutes:            make(map[types.NamespacedName]*v1.HTTPRoute),
		Services:              make(map[types.NamespacedName]*apiv1.Service),
		Namespaces:            make(map[types.NamespacedName]*apiv1.Namespace),
		ReferenceGrants:       make(map[types.NamespacedName]*v1.ReferenceGrant),
		Secrets:               make(map[types.NamespacedName]*apiv1.Secret),
		CRDMetadata:           make(map[types.NamespacedName]*metav1.PartialObjectMetadata),
		BackendTLSPolicies:    make(map[types.NamespacedName]*v1.BackendTLSPolicy),
		ConfigMaps:            make(map[types.NamespacedName]*apiv1.ConfigMap),
		NginxProxies:          make(map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy),
		GRPCRoutes:            make(map[types.NamespacedName]*v1.GRPCRoute),
		TLSRoutes:             make(map[types.NamespacedName]*v1.TLSRoute),
		TCPRoutes:             make(map[types.NamespacedName]*v1alpha2.TCPRoute),
		UDPRoutes:             make(map[types.NamespacedName]*v1alpha2.UDPRoute),
		NGFPolicies:           make(map[graph.PolicyKey]policies.Policy),
		SnippetsFilters:       make(map[types.NamespacedName]*ngfAPIv1alpha1.SnippetsFilter),
		AuthenticationFilters: make(map[types.NamespacedName]*ngfAPIv1alpha1.AuthenticationFilter),
		InferencePools:        make(map[types.NamespacedName]*inference.InferencePool),
	}

	processor := &ChangeProcessorImpl{
		cfg:          cfg,
		clusterState: clusterStore,
	}

	isReferenced := func(obj ngftypes.ObjectType, nsname types.NamespacedName) bool {
		return processor.latestGraph != nil && processor.latestGraph.IsReferenced(obj, nsname)
	}

	isNGFPolicyRelevant := func(obj ngftypes.ObjectType, nsname types.NamespacedName) bool {
		pol, ok := obj.(policies.Policy)
		if !ok {
			return false
		}

		gvk := cfg.MustExtractGVK(obj)

		return processor.latestGraph != nil && processor.latestGraph.IsNGFPolicyRelevant(pol, gvk, nsname)
	}

	// Use this object store for all NGF policies
	commonPolicyObjectStore := newNGFPolicyObjectStore(clusterStore.NGFPolicies, cfg.MustExtractGVK)

	trackingUpdaterCfg := []changeTrackingUpdaterObjectTypeCfg{
		{
			gvk:       cfg.MustExtractGVK(&v1.GatewayClass{}),
			store:     newObjectStoreMapAdapter(clusterStore.GatewayClasses),
			predicate: nil,
		},
		{
			gvk:       cfg.MustExtractGVK(&v1.Gateway{}),
			store:     newObjectStoreMapAdapter(clusterStore.Gateways),
			predicate: nil,
		},
		{
			gvk:       cfg.MustExtractGVK(&v1.HTTPRoute{}),
			store:     newObjectStoreMapAdapter(clusterStore.HTTPRoutes),
			predicate: nil,
		},
		refGrantTrackingCfg(cfg.MustExtractGVK, cfg.DiscoveredCRDs, clusterStore.ReferenceGrants),
		{
			gvk:       cfg.MustExtractGVK(&v1.BackendTLSPolicy{}),
			store:     newObjectStoreMapAdapter(clusterStore.BackendTLSPolicies),
			predicate: nil,
		},
		{
			gvk:       cfg.MustExtractGVK(&v1.GRPCRoute{}),
			store:     newObjectStoreMapAdapter(clusterStore.GRPCRoutes),
			predicate: nil,
		},
		{
			gvk:       cfg.MustExtractGVK(&apiv1.Namespace{}),
			store:     newObjectStoreMapAdapter(clusterStore.Namespaces),
			predicate: funcPredicate{stateChanged: isReferenced},
		},
		{
			gvk:       cfg.MustExtractGVK(&apiv1.Service{}),
			store:     newObjectStoreMapAdapter(clusterStore.Services),
			predicate: funcPredicate{stateChanged: isReferenced},
		},
		{
			gvk:       cfg.MustExtractGVK(&inference.InferencePool{}),
			store:     newObjectStoreMapAdapter(clusterStore.InferencePools),
			predicate: funcPredicate{stateChanged: isReferenced},
		},
		{
			gvk:       cfg.MustExtractGVK(&discoveryV1.EndpointSlice{}),
			store:     nil,
			predicate: funcPredicate{stateChanged: isReferenced},
		},
		{
			gvk:       cfg.MustExtractGVK(&apiv1.Secret{}),
			store:     newObjectStoreMapAdapter(clusterStore.Secrets),
			predicate: funcPredicate{stateChanged: isReferenced},
		},
		{
			gvk:       cfg.MustExtractGVK(&apiv1.ConfigMap{}),
			store:     newObjectStoreMapAdapter(clusterStore.ConfigMaps),
			predicate: funcPredicate{stateChanged: isReferenced},
		},
		{
			gvk:       cfg.MustExtractGVK(&apiext.CustomResourceDefinition{}),
			store:     newObjectStoreMapAdapter(clusterStore.CRDMetadata),
			predicate: annotationChangedPredicate{annotation: consts.BundleVersionAnnotation},
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha2.NginxProxy{}),
			store:     newObjectStoreMapAdapter(clusterStore.NginxProxies),
			predicate: funcPredicate{stateChanged: isReferenced},
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.ClientSettingsPolicy{}),
			store:     commonPolicyObjectStore,
			predicate: funcPredicate{stateChanged: isNGFPolicyRelevant},
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha2.ObservabilityPolicy{}),
			store:     commonPolicyObjectStore,
			predicate: funcPredicate{stateChanged: isNGFPolicyRelevant},
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.UpstreamSettingsPolicy{}),
			store:     commonPolicyObjectStore,
			predicate: funcPredicate{stateChanged: isNGFPolicyRelevant},
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.ProxySettingsPolicy{}),
			store:     commonPolicyObjectStore,
			predicate: funcPredicate{stateChanged: isNGFPolicyRelevant},
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.WAFPolicy{}),
			store:     commonPolicyObjectStore,
			predicate: funcPredicate{stateChanged: isNGFPolicyRelevant},
		},
		{
			gvk:       cfg.MustExtractGVK(&v1.TLSRoute{}),
			store:     newObjectStoreMapAdapter(clusterStore.TLSRoutes),
			predicate: nil,
		},
		{
			gvk:       cfg.MustExtractGVK(&v1alpha2.TCPRoute{}),
			store:     newObjectStoreMapAdapter(clusterStore.TCPRoutes),
			predicate: nil,
		},
		{
			gvk:       cfg.MustExtractGVK(&v1alpha2.UDPRoute{}),
			store:     newObjectStoreMapAdapter(clusterStore.UDPRoutes),
			predicate: nil,
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.SnippetsFilter{}),
			store:     newObjectStoreMapAdapter(clusterStore.SnippetsFilters),
			predicate: nil, // we always want to write status to SnippetsFilters so we don't filter them out
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.AuthenticationFilter{}),
			store:     newObjectStoreMapAdapter(clusterStore.AuthenticationFilters),
			predicate: nil, // we always want to write status to AuthenticationFilters so we don't filter them out
		},
		{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.RateLimitPolicy{}),
			store:     commonPolicyObjectStore,
			predicate: funcPredicate{stateChanged: isNGFPolicyRelevant},
		},
	}

	if cfg.Snippets {
		trackingUpdaterCfg = append(trackingUpdaterCfg, changeTrackingUpdaterObjectTypeCfg{
			gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.SnippetsPolicy{}),
			store:     commonPolicyObjectStore,
			predicate: funcPredicate{stateChanged: isNGFPolicyRelevant},
		})
	}

	trackingUpdater := newChangeTrackingUpdater(
		cfg.MustExtractGVK,
		trackingUpdaterCfg,
	)

	processor.getAndResetClusterStateChanged = trackingUpdater.getAndResetChangedStatus
	processor.forceClusterStateRebuild = trackingUpdater.forceRebuild
	processor.updater = trackingUpdater

	return processor
}

// Currently, changes (upserts/delete) trigger rebuilding of the configuration, even if the change doesn't change
// the configuration or the statuses of the resources. For example, a change in a Gateway resource that doesn't
// belong to the NGINX Gateway Fabric or an HTTPRoute that doesn't belong to any of the Gateways of the
// NGINX Gateway Fabric. Find a way to ignore changes that don't affect the configuration and/or statuses of
// the resources.
// Tracking issues: https://github.com/nginx/nginx-gateway-fabric/issues/1123,
// https://github.com/nginx/nginx-gateway-fabric/issues/1124,
// https://github.com/nginx/nginx-gateway-fabric/issues/1577

// FIXME(pleshakov)
// Remove CaptureUpsertChange() and CaptureDeleteChange() from ChangeProcessor and pass all changes directly to
// Process() instead. As a result, the clients will only need to call Process(), which will simplify them.
// Now the clients make a combination of CaptureUpsertChange() and CaptureDeleteChange() calls followed by a call to
// Process().

func (c *ChangeProcessorImpl) CaptureUpsertChange(obj client.Object) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.updater.Upsert(obj)
}

func (c *ChangeProcessorImpl) CaptureDeleteChange(resourceType ngftypes.ObjectType, nsname types.NamespacedName) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.updater.Delete(resourceType, nsname)
}

// ForceRebuild forces the next Process() call to rebuild the graph without modifying cluster state.
func (c *ChangeProcessorImpl) ForceRebuild() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.forceClusterStateRebuild()
}

func (c *ChangeProcessorImpl) Process(ctx context.Context) *graph.Graph {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.getAndResetClusterStateChanged() {
		return nil
	}

	previousWAFBundles := c.mergedWAFBundles()

	c.latestGraph = graph.BuildGraph(
		ctx,
		c.clusterState,
		c.cfg.GatewayCtlrName,
		c.cfg.GatewayClassName,
		c.cfg.PlusSecrets,
		c.cfg.WAFFetcher,
		previousWAFBundles,
		c.cfg.Validators,
		c.cfg.Logger,
		c.cfg.FeatureFlags,
	)

	return c.latestGraph
}

// mergedWAFBundles combines graph-cached bundles with any fresher bundles from WAF pollers.
// Polled bundles take precedence because they may be newer than what the graph last stored.
// This prevents a graph rebuild from overwriting polled data with stale cached data
// when a re-fetch fails.
func (c *ChangeProcessorImpl) mergedWAFBundles() map[graph.WAFBundleKey]*graph.WAFBundleData {
	var graphBundles map[graph.WAFBundleKey]*graph.WAFBundleData
	if c.latestGraph != nil {
		graphBundles = c.latestGraph.ReferencedWAFBundles
	}

	var polledBundles map[graph.WAFBundleKey]*graph.WAFBundleData
	if c.cfg.PolledWAFBundles != nil {
		polledBundles = c.cfg.PolledWAFBundles()
	}

	if len(graphBundles) == 0 && len(polledBundles) == 0 {
		return nil
	}

	merged := make(map[graph.WAFBundleKey]*graph.WAFBundleData, len(graphBundles)+len(polledBundles))

	// Start with graph-cached bundles.
	maps.Copy(merged, graphBundles)

	// Overlay polled bundles — these are newer and take precedence.
	maps.Copy(merged, polledBundles)

	return merged
}

func (c *ChangeProcessorImpl) GetLatestGraph() *graph.Graph {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.latestGraph
}

// refGrantTrackingCfg returns the change tracking updater config for ReferenceGrant.
// If v1 ReferenceGrant CRD exists in the cluster (or discoveredCRDs is nil, i.e. not populated),
// it tracks the v1 GVK directly.
// If v1 is explicitly not found, it falls back to tracking v1beta1 and converting objects to v1 before storing.
func refGrantTrackingCfg(
	mustExtractGVK kinds.MustExtractGVK,
	discoveredCRDs map[string]bool,
	refGrants map[types.NamespacedName]*v1.ReferenceGrant,
) changeTrackingUpdaterObjectTypeCfg {
	// Use v1beta1 only when we've explicitly checked and v1 is not available.
	exists, checked := discoveredCRDs[kinds.ReferenceGrant]
	if checked && !exists {
		return changeTrackingUpdaterObjectTypeCfg{
			gvk:       mustExtractGVK(&v1beta1.ReferenceGrant{}),
			store:     newConvertingReferenceGrantStore(refGrants),
			predicate: nil,
		}
	}

	return changeTrackingUpdaterObjectTypeCfg{
		gvk:       mustExtractGVK(&v1.ReferenceGrant{}),
		store:     newObjectStoreMapAdapter(refGrants),
		predicate: nil,
	}
}
