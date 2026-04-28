package poller

import (
	"context"
	"fmt"
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
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch"
)

// defaultPollingInterval is the default interval between poll cycles.
const defaultPollingInterval = 5 * time.Minute

type BundleType int

const (
	PolicyBundle BundleType = iota
	LogProfileBundle
)

// BundleSource represents a single bundle source that needs polling.
// This can be either the main policy bundle or a log bundle.
type BundleSource struct {
	// BundleKey is the unique identifier for this bundle.
	BundleKey graph.WAFBundleKey
	// Request contains the fetch configuration for this bundle.
	Request fetch.Request
	// Type indicates whether this is a policy bundle or a log profile bundle, which determines the fetch method used.
	Type BundleType
	// Interval is the polling interval for this source.
	Interval time.Duration
}

// bundleState tracks the last known state of a fetched bundle, including the checksum and any
// conditional-request token (ETag or Last-Modified) for use on subsequent HTTP polls.
type bundleState struct {
	checksum         string
	conditionalToken string // ETag or Last-Modified from the last successful HTTP fetch
}

// poller handles periodic re-fetching of WAF bundles for a single WAFPolicy.
// It compares checksums to detect changes and pushes updated bundles to relevant deployments.
type poller struct {
	fetcher           fetch.Fetcher
	deployments       agent.DeploymentStorer
	targetDeployments map[types.NamespacedName]struct{}
	bundleStates      map[graph.WAFBundleKey]bundleState
	statusCallback    func(
		policyNsName types.NamespacedName,
		bundleKey graph.WAFBundleKey,
		newChecksum string,
		err error,
	)
	bundleUpdateCallback func(bundleKey graph.WAFBundleKey, data []byte, checksum string)
	policyNsName         types.NamespacedName
	logger               logr.Logger
	sources              []BundleSource
	targetMu             sync.RWMutex
	stateMu              sync.RWMutex
}

// pollerConfig contains the configuration for creating a new poller.
type pollerConfig struct {
	fetcher          fetch.Fetcher
	deployments      agent.DeploymentStorer
	initialChecksums map[graph.WAFBundleKey]string
	statusCallback   func(
		policyNsName types.NamespacedName,
		bundleKey graph.WAFBundleKey,
		newChecksum string,
		err error,
	)
	bundleUpdateCallback func(bundleKey graph.WAFBundleKey, data []byte, checksum string)
	policyNsName         types.NamespacedName
	logger               logr.Logger
	sources              []BundleSource
	targetDeployments    []types.NamespacedName
}

// newPoller creates a new poller for the given WAFPolicy.
func newPoller(cfg pollerConfig) *poller {
	targets := make(map[types.NamespacedName]struct{}, len(cfg.targetDeployments))
	for _, t := range cfg.targetDeployments {
		targets[t] = struct{}{}
	}

	states := make(map[graph.WAFBundleKey]bundleState, len(cfg.initialChecksums))
	for k, cs := range cfg.initialChecksums {
		states[k] = bundleState{checksum: cs}
	}

	return &poller{
		logger:               cfg.logger.WithValues("policy", cfg.policyNsName),
		policyNsName:         cfg.policyNsName,
		sources:              cfg.sources,
		fetcher:              cfg.fetcher,
		deployments:          cfg.deployments,
		targetDeployments:    targets,
		bundleStates:         states,
		statusCallback:       cfg.statusCallback,
		bundleUpdateCallback: cfg.bundleUpdateCallback,
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

	if minInterval <= 0 {
		p.logger.Error(nil, fmt.Sprintf(
			"Invalid polling interval, must be greater than zero. Using default interval: %v", defaultPollingInterval,
		))
		minInterval = defaultPollingInterval
	}

	p.logger.Info("WAF polling started", "interval", minInterval, "sourceCount", len(p.sources))

	// Track last poll time for each source to handle different intervals.
	lastPoll := make(map[graph.WAFBundleKey]time.Time, len(p.sources))

	// Poll all sources immediately on startup, then start the ticker for subsequent polls.
	now := time.Now()
	for _, src := range p.sources {
		p.pollSource(ctx, src)
		lastPoll[src.BundleKey] = now
	}

	ticker := time.NewTicker(minInterval)
	defer ticker.Stop()

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
//
// For NIM and N1C sources a two-phase approach is used: first only the remote checksum is
// retrieved, and the full bundle is downloaded only when the checksum differs from the last
// known value.
//
// For plain HTTP sources a conditional GET is issued using any ETag or Last-Modified token
// stored from the previous successful fetch. A 304 Not Modified response is treated as
// unchanged without downloading the bundle.
func (p *poller) pollSource(ctx context.Context, src BundleSource) {
	p.logger.V(1).Info("Polling bundle source")

	p.stateMu.RLock()
	last := p.bundleStates[src.BundleKey]
	p.stateMu.RUnlock()

	if src.Request.SupportsChecksumOnlyFetch() {
		if skip := p.skipIfChecksumUnchanged(ctx, src, last.checksum); skip {
			return
		}
	}

	result, err := p.downloadBundle(ctx, src, last)
	if err != nil || result.Unchanged || result.Checksum == last.checksum {
		p.reportStatus(src.BundleKey, "", err)
		return
	}

	p.logger.Info("Bundle changed, pushing to deployments", "newChecksum", result.Checksum)
	p.pushBundleToDeployments(src.BundleKey, result.Data)
	p.saveBundleState(src.BundleKey, result)

	if p.bundleUpdateCallback != nil {
		p.bundleUpdateCallback(src.BundleKey, result.Data, result.Checksum)
	}
	p.reportStatus(src.BundleKey, result.Checksum, nil)
}

// skipIfChecksumUnchanged fetches only the remote checksum for a NIM or N1C source.
// It reports whether polling should skip the full download (true = skip). When skipping due to
// an error or an unchanged checksum the appropriate status callback is fired.
func (p *poller) skipIfChecksumUnchanged(ctx context.Context, src BundleSource, lastChecksum string) bool {
	changed, checksum, err := p.checksumChanged(ctx, src, lastChecksum)
	if err != nil {
		p.logger.Error(err, "Failed to fetch bundle checksum during poll")
		p.reportStatus(src.BundleKey, "", err)
		return true
	}
	if !changed {
		p.logger.V(1).Info("Bundle unchanged, skipping download")
		p.reportStatus(src.BundleKey, "", nil)
		return true
	}
	p.logger.Info("Bundle checksum changed, downloading full bundle", "newChecksum", checksum)
	return false
}

// downloadBundle fetches the full bundle for src, attaching the stored conditional token so that
// HTTP servers can respond with 304 Not Modified when nothing has changed.
func (p *poller) downloadBundle(
	ctx context.Context, src BundleSource, last bundleState,
) (fetch.Result, error) {
	req := src.Request
	req.ConditionalToken = last.conditionalToken

	result, err := p.fetchBundle(ctx, src, req)
	if err != nil {
		p.logger.Error(err, "Failed to fetch bundle during poll")
		return fetch.Result{}, err
	}
	if result.Unchanged {
		p.logger.V(1).Info("Bundle unchanged (304), skipping push")
	} else if result.Checksum == last.checksum {
		p.logger.V(1).Info("Bundle unchanged, skipping push")
	}
	return result, nil
}

// saveBundleState persists the new checksum and HTTP conditional token after a successful push.
func (p *poller) saveBundleState(bundleKey graph.WAFBundleKey, result fetch.Result) {
	newState := bundleState{checksum: result.Checksum}
	if result.ETag != "" {
		newState.conditionalToken = result.ETag
	} else if result.LastModified != "" {
		newState.conditionalToken = result.LastModified
	}
	p.stateMu.Lock()
	p.bundleStates[bundleKey] = newState
	p.stateMu.Unlock()
}

// reportStatus fires the status callback if one is registered.
// newChecksum is non-empty when the bundle was successfully updated; empty for unchanged or error cases.
func (p *poller) reportStatus(bundleKey graph.WAFBundleKey, newChecksum string, err error) {
	if p.statusCallback != nil {
		p.statusCallback(p.policyNsName, bundleKey, newChecksum, err)
	}
}

// checksumChanged fetches only the remote checksum for a NIM or N1C source and reports whether
// it differs from lastChecksum. Returns (changed, remoteChecksum, error).
func (p *poller) checksumChanged(ctx context.Context, src BundleSource, lastChecksum string) (bool, string, error) {
	var checksum string
	var err error

	if src.Type == LogProfileBundle {
		checksum, err = p.fetcher.FetchLogProfileBundleChecksum(ctx, src.Request)
	} else {
		checksum, err = p.fetcher.FetchPolicyBundleChecksum(ctx, src.Request)
	}
	if err != nil {
		return false, "", err
	}

	return checksum != lastChecksum, checksum, nil
}

// fetchBundle downloads the full bundle for the given source using req (which may carry a
// conditional token for HTTP sources).
func (p *poller) fetchBundle(ctx context.Context, src BundleSource, req fetch.Request) (fetch.Result, error) {
	if src.Type == LogProfileBundle {
		return p.fetcher.FetchLogProfileBundle(ctx, req)
	}
	return p.fetcher.FetchPolicyBundle(ctx, req)
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

// BuildBundleSources constructs BundleSource entries from a WAFPolicy spec.
// It returns only sources that have polling enabled.
func BuildBundleSources(
	policyNsName types.NamespacedName,
	spec ngfAPIv1alpha1.WAFPolicySpec,
	auth *fetch.BundleAuth,
	tlsCA []byte,
) []BundleSource {
	var sources []BundleSource

	// Check if policySource has polling enabled.
	if spec.PolicySource.Polling != nil && spec.PolicySource.Polling.Enabled {
		interval := defaultPollingInterval
		if spec.PolicySource.Polling.Interval != nil && spec.PolicySource.Polling.Interval.Duration > 0 {
			interval = spec.PolicySource.Polling.Interval.Duration
		}

		sources = append(sources, BundleSource{
			Type:      PolicyBundle,
			BundleKey: graph.PolicyBundleKey(policyNsName),
			Request:   graph.BuildPolicyFetchRequest(&spec.PolicySource, spec.Type, auth, tlsCA),
			Interval:  interval,
		})
	}

	// Check each logSource for polling.
	for _, secLog := range spec.SecurityLogs {
		if secLog.LogSource.HTTPSource == nil && secLog.LogSource.NIMSource == nil && secLog.LogSource.N1CSource == nil {
			continue // DefaultProfile, no polling needed.
		}
		if secLog.LogSource.Polling == nil || !secLog.LogSource.Polling.Enabled {
			continue
		}

		interval := defaultPollingInterval
		if secLog.LogSource.Polling.Interval != nil && secLog.LogSource.Polling.Interval.Duration > 0 {
			interval = secLog.LogSource.Polling.Interval.Duration
		}

		sources = append(sources, BundleSource{
			Type:      LogProfileBundle,
			BundleKey: graph.LogBundleKey(policyNsName, &secLog.LogSource),
			Request:   graph.BuildLogFetchRequest(&secLog.LogSource, auth, tlsCA),
			Interval:  interval,
		})
	}

	return sources
}
