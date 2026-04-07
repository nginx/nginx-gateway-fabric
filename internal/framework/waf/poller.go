package waf

import (
	"context"
	"maps"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
)

// defaultPollingInterval is the default interval between poll cycles.
const defaultPollingInterval = 5 * time.Minute

// BundleSource represents a single bundle source that needs polling.
// This can be either the main policy bundle or a log bundle.
type BundleSource struct {
	// BundleKey is the unique identifier for this bundle.
	BundleKey graph.WAFBundleKey
	// Request contains the fetch configuration for this bundle.
	Request fetch.Request
	// Interval is the polling interval for this source.
	Interval time.Duration
}

// poller handles periodic re-fetching of WAF bundles for a single WAFGatewayBindingPolicy.
// It compares checksums to detect changes and pushes updated bundles to relevant deployments.
type poller struct {
	fetcher           fetch.Fetcher
	deployments       agent.DeploymentStorer
	targetDeployments map[types.NamespacedName]struct{}
	lastChecksums     map[graph.WAFBundleKey]string
	statusCallback    func(policyNsName types.NamespacedName, bundleKey graph.WAFBundleKey, err error)
	policyNsName      types.NamespacedName
	logger            logr.Logger
	sources           []BundleSource
	targetMu          sync.RWMutex
	checksumMu        sync.RWMutex
}

// pollerConfig contains the configuration for creating a new poller.
type pollerConfig struct {
	fetcher           fetch.Fetcher
	deployments       agent.DeploymentStorer
	initialChecksums  map[graph.WAFBundleKey]string
	statusCallback    func(policyNsName types.NamespacedName, bundleKey graph.WAFBundleKey, err error)
	policyNsName      types.NamespacedName
	logger            logr.Logger
	sources           []BundleSource
	targetDeployments []types.NamespacedName
}

// newPoller creates a new poller for the given WAFGatewayBindingPolicy.
func newPoller(cfg pollerConfig) *poller {
	targets := make(map[types.NamespacedName]struct{}, len(cfg.targetDeployments))
	for _, t := range cfg.targetDeployments {
		targets[t] = struct{}{}
	}

	checksums := make(map[graph.WAFBundleKey]string, len(cfg.initialChecksums))
	maps.Copy(checksums, cfg.initialChecksums)

	return &poller{
		logger:            cfg.logger.WithValues("policy", cfg.policyNsName),
		policyNsName:      cfg.policyNsName,
		sources:           cfg.sources,
		fetcher:           cfg.fetcher,
		deployments:       cfg.deployments,
		targetDeployments: targets,
		lastChecksums:     checksums,
		statusCallback:    cfg.statusCallback,
	}
}

// run starts the polling loop. It blocks until the context is canceled.
func (p *poller) run(ctx context.Context) {
	if len(p.sources) == 0 {
		p.logger.V(1).Info("No sources with polling enabled, poller exiting")
		return
	}

	// Find the minimum interval among all sources to use as the tick interval.
	minInterval := p.sources[0].Interval
	for _, src := range p.sources[1:] {
		minInterval = min(minInterval, src.Interval)
	}

	ticker := time.NewTicker(minInterval)
	defer ticker.Stop()

	// Track last poll time for each source to handle different intervals.
	lastPoll := make(map[graph.WAFBundleKey]time.Time, len(p.sources))
	now := time.Now()
	for _, src := range p.sources {
		// Initialize to now minus interval so first tick triggers immediately.
		lastPoll[src.BundleKey] = now.Add(-src.Interval)
	}

	p.logger.Info("WAF polling started", "interval", minInterval, "sourceCount", len(p.sources))

	for {
		select {
		case <-ctx.Done():
			p.logger.V(1).Info("Poller stopping due to context cancellation")
			return
		case now := <-ticker.C:
			for _, src := range p.sources {
				if now.Sub(lastPoll[src.BundleKey]) >= src.Interval {
					p.pollSource(ctx, src)
					lastPoll[src.BundleKey] = now
				}
			}
		}
	}
}

// getTargetDeployments returns the current set of target deployment names.
func (p *poller) getTargetDeployments() []types.NamespacedName {
	p.targetMu.RLock()
	defer p.targetMu.RUnlock()

	targets := make([]types.NamespacedName, 0, len(p.targetDeployments))
	for t := range p.targetDeployments {
		targets = append(targets, t)
	}

	return targets
}

// updateTargetDeployments updates the set of deployments this poller should push bundles to.
func (p *poller) updateTargetDeployments(targets []types.NamespacedName) {
	newTargets := make(map[types.NamespacedName]struct{}, len(targets))
	for _, t := range targets {
		newTargets[t] = struct{}{}
	}

	p.targetMu.Lock()
	defer p.targetMu.Unlock()

	p.targetDeployments = newTargets
}

// getSources returns the bundle sources this poller is monitoring.
func (p *poller) getSources() []BundleSource {
	return p.sources
}

// sourcesEqual returns true if two BundleSource slices are equivalent.
// This is used to determine if a poller needs to be restarted due to source changes.
func sourcesEqual(a, b []BundleSource) bool {
	return reflect.DeepEqual(a, b)
}

// pollSource fetches a single bundle source and pushes it to deployments if changed.
func (p *poller) pollSource(ctx context.Context, src BundleSource) {
	p.logger.V(1).Info("Polling bundle source")

	data, checksum, err := p.fetcher.Fetch(ctx, src.Request)
	if err != nil {
		p.logger.Error(err, "Failed to fetch bundle during poll")
		if p.statusCallback != nil {
			p.statusCallback(p.policyNsName, src.BundleKey, err)
		}
		// Keep existing bundle active, retry on next interval.
		return
	}

	// Compare checksum to detect changes.
	p.checksumMu.RLock()
	lastChecksum := p.lastChecksums[src.BundleKey]
	p.checksumMu.RUnlock()

	if checksum == lastChecksum {
		p.logger.V(1).Info("Bundle unchanged, skipping push")
		return
	}

	p.logger.Info("Bundle changed, pushing to deployments", "newChecksum", checksum)

	// Push to all target deployments.
	p.pushBundleToDeployments(src.BundleKey, data)

	// Update stored checksum.
	p.checksumMu.Lock()
	p.lastChecksums[src.BundleKey] = checksum
	p.checksumMu.Unlock()

	if p.statusCallback != nil {
		p.statusCallback(p.policyNsName, src.BundleKey, nil)
	}
}

// pushBundleToDeployments pushes the bundle to all target deployments.
func (p *poller) pushBundleToDeployments(bundleKey graph.WAFBundleKey, data []byte) {
	p.targetMu.RLock()
	defer p.targetMu.RUnlock()

	bundlePath := config.GenerateWAFBundleFileName(dataplane.WAFBundleID(bundleKey))

	for depName := range p.targetDeployments {
		deployment := p.deployments.Get(depName)
		if deployment == nil {
			p.logger.V(1).Info("Deployment not found, skipping bundle push", "deployment", depName)
			continue
		}

		deployment.FileLock.Lock()
		msg := deployment.UpdateWAFBundle(bundlePath, data)
		if msg != nil {
			applied := deployment.GetBroadcaster().Send(*msg)
			if applied {
				p.logger.Info(
					"Pushed updated WAF bundle to deployment",
					"deployment", depName,
				)
			} else {
				p.logger.V(1).Info(
					"No subscribers for deployment, bundle stored but not pushed",
					"deployment", depName,
				)
			}
		}
		deployment.FileLock.Unlock()
	}
}

// BuildBundleSources constructs BundleSource entries from a WAFGatewayBindingPolicy spec.
// It returns only sources that have polling enabled.
func BuildBundleSources(
	policyNsName types.NamespacedName,
	spec ngfAPIv1alpha1.WAFGatewayBindingPolicySpec,
	auth *fetch.BundleAuth,
	tlsCA []byte,
) []BundleSource {
	var sources []BundleSource

	// Check if policySource has polling enabled.
	if spec.PolicySource.Polling != nil && spec.PolicySource.Polling.Enabled {
		interval := defaultPollingInterval
		if spec.PolicySource.Polling.Interval != nil {
			interval = spec.PolicySource.Polling.Interval.Duration
		}

		sources = append(sources, BundleSource{
			BundleKey: graph.PolicyBundleKey(policyNsName),
			Request:   graph.BuildPolicyFetchRequest(&spec.PolicySource, spec.Type, auth, tlsCA),
			Interval:  interval,
		})
	}

	// Check each logSource for polling.
	for _, secLog := range spec.SecurityLogs {
		if secLog.LogSource.URL == nil {
			continue // DefaultProfile, no polling needed.
		}
		if secLog.LogSource.Polling == nil || !secLog.LogSource.Polling.Enabled {
			continue
		}

		interval := defaultPollingInterval
		if secLog.LogSource.Polling.Interval != nil {
			interval = secLog.LogSource.Polling.Interval.Duration
		}

		sources = append(sources, BundleSource{
			BundleKey: graph.LogBundleKey(policyNsName, *secLog.LogSource.URL),
			Request:   graph.BuildLogFetchRequest(&secLog.LogSource, auth, tlsCA),
			Interval:  interval,
		})
	}

	return sources
}
