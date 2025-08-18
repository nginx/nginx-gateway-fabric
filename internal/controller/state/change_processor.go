package state

import (
	"sync"

	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
	ngftypes "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
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
	Process() (graphCfg *graph.Graph)
	// GetLatestGraph returns the latest Graph.
	GetLatestGraph() *graph.Graph
}

// ChangeProcessorConfig holds configuration parameters for ChangeProcessorImpl.
type ChangeProcessorConfig struct {
	// Validators validate resources according to data-plane specific rules.
	Validators validation.Validators
	// EventRecorder records events for Kubernetes resources.
	EventRecorder record.EventRecorder
	// MustExtractGVK is a function that extracts schema.GroupVersionKind from a client.Object.
	MustExtractGVK kinds.MustExtractGVK
	// PlusSecrets is a list of secret files used for NGINX Plus reporting (JWT, client SSL, CA).
	PlusSecrets map[types.NamespacedName][]graph.PlusSecretFile
	// Logger is the logger for this Change Processor.
	Logger logr.Logger
	// GatewayCtlrName is the name of the Gateway controller.
	GatewayCtlrName string
	// GatewayClassName is the name of the GatewayClass resource.
	GatewayClassName string
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

	cfg  ChangeProcessorConfig
	lock sync.Mutex
}

// NewChangeProcessorImpl creates a new ChangeProcessorImpl for the Gateway resource with the configured namespace name.
func NewChangeProcessorImpl(cfg ChangeProcessorConfig) *ChangeProcessorImpl {
	clusterStore := graph.ClusterState{
		GatewayClasses:     make(map[types.NamespacedName]*v1.GatewayClass),
		Gateways:           make(map[types.NamespacedName]*v1.Gateway),
		HTTPRoutes:         make(map[types.NamespacedName]*v1.HTTPRoute),
		Services:           make(map[types.NamespacedName]*apiv1.Service),
		Namespaces:         make(map[types.NamespacedName]*apiv1.Namespace),
		ReferenceGrants:    make(map[types.NamespacedName]*v1beta1.ReferenceGrant),
		Secrets:            make(map[types.NamespacedName]*apiv1.Secret),
		CRDMetadata:        make(map[types.NamespacedName]*metav1.PartialObjectMetadata),
		BackendTLSPolicies: make(map[types.NamespacedName]*v1alpha3.BackendTLSPolicy),
		ConfigMaps:         make(map[types.NamespacedName]*apiv1.ConfigMap),
		NginxProxies:       make(map[types.NamespacedName]*ngfAPIv1alpha2.NginxProxy),
		GRPCRoutes:         make(map[types.NamespacedName]*v1.GRPCRoute),
		TLSRoutes:          make(map[types.NamespacedName]*v1alpha2.TLSRoute),
		NGFPolicies:        make(map[graph.PolicyKey]policies.Policy),
		SnippetsFilters:    make(map[types.NamespacedName]*ngfAPIv1alpha1.SnippetsFilter),
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

	trackingUpdater := newChangeTrackingUpdater(
		cfg.MustExtractGVK,
		[]changeTrackingUpdaterObjectTypeCfg{
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
			{
				gvk:       cfg.MustExtractGVK(&v1beta1.ReferenceGrant{}),
				store:     newObjectStoreMapAdapter(clusterStore.ReferenceGrants),
				predicate: nil,
			},
			{
				gvk:       cfg.MustExtractGVK(&v1alpha3.BackendTLSPolicy{}),
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
				predicate: annotationChangedPredicate{annotation: graph.BundleVersionAnnotation},
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
				gvk:       cfg.MustExtractGVK(&v1alpha2.TLSRoute{}),
				store:     newObjectStoreMapAdapter(clusterStore.TLSRoutes),
				predicate: nil,
			},
			{
				gvk:       cfg.MustExtractGVK(&ngfAPIv1alpha1.SnippetsFilter{}),
				store:     newObjectStoreMapAdapter(clusterStore.SnippetsFilters),
				predicate: nil, // we always want to write status to SnippetsFilters so we don't filter them out
			},
		},
	)

	processor.getAndResetClusterStateChanged = trackingUpdater.getAndResetChangedStatus
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

func (c *ChangeProcessorImpl) Process() *graph.Graph {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.getAndResetClusterStateChanged() {
		return nil
	}

	c.latestGraph = graph.BuildGraph(
		c.clusterState,
		c.cfg.GatewayCtlrName,
		c.cfg.GatewayClassName,
		c.cfg.PlusSecrets,
		c.cfg.Validators,
		c.cfg.Logger,
	)

	return c.latestGraph
}

func (c *ChangeProcessorImpl) GetLatestGraph() *graph.Graph {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.latestGraph
}
