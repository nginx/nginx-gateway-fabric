package waf

import (
	"context"
	"maps"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/fetch"
)

//go:generate go tool counterfeiter -generate

// PollError represents a polling error for a specific bundle.
type PollError struct {
	Err       error
	BundleKey graph.WAFBundleKey
}

// PollerManager is the interface for managing WAF bundle pollers.
//
//counterfeiter:generate . PollerManager
type PollerManager interface {
	// ReconcilePoller ensures a poller is running with the correct configuration.
	ReconcilePoller(ctx context.Context, cfg PollerConfig)
	// GetAllPollErrors returns a deep copy of all current poll errors.
	GetAllPollErrors() map[types.NamespacedName]PollError
	// GetLatestBundles returns a copy of all bundles that have been successfully fetched by pollers.
	// These represent the freshest known bundle data and should take precedence over
	// graph-cached bundles when constructing stale-bundle fallback state.
	GetLatestBundles() map[graph.WAFBundleKey]*graph.WAFBundleData
	// StopPoller stops the poller for a WAFGatewayBindingPolicy.
	StopPoller(policyNsName types.NamespacedName)
	// StopPollersNotIn stops all pollers whose policy namespace/name is not in the given set.
	StopPollersNotIn(activePolicies map[types.NamespacedName]struct{})
}

// Manager manages the lifecycle of all WAF bundle pollers.
// It creates, tracks, and stops pollers as WAFGatewayBindingPolicies are created, updated, or deleted.
type Manager struct {
	fetcher        fetch.Fetcher
	deployments    agent.DeploymentStorer
	pollers        map[types.NamespacedName]*pollerEntry
	pollErrors     map[types.NamespacedName]*PollError
	bundleCache    map[graph.WAFBundleKey]*graph.WAFBundleData
	statusCallback func(targets []types.NamespacedName)
	logger         logr.Logger
	mu             sync.RWMutex
}

// pollerEntry holds a poller and its cancellation function.
type pollerEntry struct {
	poller *poller
	cancel context.CancelFunc
}

// ManagerConfig contains configuration for creating a new Manager.
type ManagerConfig struct {
	Fetcher        fetch.Fetcher
	Deployments    agent.DeploymentStorer
	StatusCallback func(targets []types.NamespacedName)
	Logger         logr.Logger
}

// NewManager creates a new Manager.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{
		logger:         cfg.Logger,
		fetcher:        cfg.Fetcher,
		deployments:    cfg.Deployments,
		pollers:        make(map[types.NamespacedName]*pollerEntry),
		pollErrors:     make(map[types.NamespacedName]*PollError),
		bundleCache:    make(map[graph.WAFBundleKey]*graph.WAFBundleData),
		statusCallback: cfg.StatusCallback,
	}
}

// PollerConfig contains configuration for reconciling a poller.
type PollerConfig struct {
	InitialChecksums  map[graph.WAFBundleKey]string
	PolicyNsName      types.NamespacedName
	Sources           []BundleSource
	TargetDeployments []types.NamespacedName
}

// ReconcilePoller ensures a poller is running with the correct configuration.
// If no poller exists, one is started. If a poller exists with the same sources,
// only the target deployments are updated. If sources have changed, the poller is restarted.
// This avoids unnecessary poller restarts when only targets change.
func (m *Manager) ReconcilePoller(ctx context.Context, cfg PollerConfig) {
	if len(cfg.Sources) == 0 {
		m.logger.V(1).Info("No polling sources, not starting poller", "policy", cfg.PolicyNsName)
		return
	}

	m.mu.Lock()
	entry, exists := m.pollers[cfg.PolicyNsName]

	// If poller exists and sources haven't changed, just update targets if needed.
	if exists && sourcesEqual(entry.poller.getSources(), cfg.Sources) {
		m.mu.Unlock()
		entry.poller.updateTargetDeployments(cfg.TargetDeployments)
		return
	}
	m.mu.Unlock()

	// Sources changed or poller doesn't exist - need to (re)start.
	m.startPoller(ctx, cfg)
}

// startPoller starts a new poller, stopping any existing one first.
func (m *Manager) startPoller(ctx context.Context, cfg PollerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Always cancel any existing poller before overwriting the map entry.
	// This is safe even if the caller didn't observe one, because another
	// goroutine may have started one between our check and acquiring this lock.
	if entry, exists := m.pollers[cfg.PolicyNsName]; exists {
		m.logger.V(1).Info("Stopping existing poller before starting new one", "policy", cfg.PolicyNsName)
		entry.cancel()
		delete(m.pollErrors, cfg.PolicyNsName)
	}

	pollerCtx, cancel := context.WithCancel(ctx) //nolint:gosec // Cancel is handled externally to this function

	var poller *poller

	// Create a wrapped callback that records poll results and triggers a status update
	// scoped to just the poller's target deployments.
	wrappedCallback := func(policyNsName types.NamespacedName, bundleKey graph.WAFBundleKey, err error) {
		m.recordPollResult(policyNsName, bundleKey, err)
		if m.statusCallback != nil {
			m.statusCallback(poller.getTargetDeployments())
		}
	}

	poller = newPoller(pollerConfig{
		logger:               m.logger,
		policyNsName:         cfg.PolicyNsName,
		sources:              cfg.Sources,
		fetcher:              m.fetcher,
		deployments:          m.deployments,
		targetDeployments:    cfg.TargetDeployments,
		initialChecksums:     cfg.InitialChecksums,
		statusCallback:       wrappedCallback,
		bundleUpdateCallback: m.cacheBundleUpdate,
	})

	m.pollers[cfg.PolicyNsName] = &pollerEntry{
		poller: poller,
		cancel: cancel,
	}

	go func() {
		poller.run(pollerCtx)
		// Clean up after poller exits.
		m.mu.Lock()
		// Only delete if this is still the same poller (could have been replaced).
		if entry, exists := m.pollers[cfg.PolicyNsName]; exists && entry.poller == poller {
			delete(m.pollers, cfg.PolicyNsName)
		}
		m.mu.Unlock()
	}()

	m.logger.Info("Started WAF poller", "policy", cfg.PolicyNsName, "sourceCount", len(cfg.Sources))
}

// recordPollResult records the result of a poll attempt for a policy.
// If err is nil, it clears any previous error only if it was for the same bundle key.
// If err is non-nil, it stores the error for this bundle key.
// This method is called by the internal status callback.
func (m *Manager) recordPollResult(policyNsName types.NamespacedName, bundleKey graph.WAFBundleKey, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err == nil {
		if existing := m.pollErrors[policyNsName]; existing != nil && existing.BundleKey == bundleKey {
			delete(m.pollErrors, policyNsName)
		}
	} else {
		m.pollErrors[policyNsName] = &PollError{
			BundleKey: bundleKey,
			Err:       err,
		}
	}
}

// cacheBundleUpdate stores the latest successfully polled bundle data in the manager's cache.
// This is called by pollers when they detect a changed bundle, ensuring the freshest data
// is available for graph rebuild stale-bundle fallback.
func (m *Manager) cacheBundleUpdate(bundleKey graph.WAFBundleKey, data []byte, checksum string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bundleCache[bundleKey] = &graph.WAFBundleData{
		Data:     data,
		Checksum: checksum,
	}
}

// GetAllPollErrors returns a deep copy of all current poll errors.
func (m *Manager) GetAllPollErrors() map[types.NamespacedName]PollError {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.pollErrors) == 0 {
		return nil
	}

	result := make(map[types.NamespacedName]PollError, len(m.pollErrors))
	for k, v := range m.pollErrors {
		result[k] = *v
	}
	return result
}

// GetLatestBundles returns a copy of all bundles that have been successfully fetched by pollers.
// These represent the freshest known bundle data and should take precedence over
// graph-cached bundles when constructing stale-bundle fallback state.
func (m *Manager) GetLatestBundles() map[graph.WAFBundleKey]*graph.WAFBundleData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.bundleCache) == 0 {
		return nil
	}

	result := make(map[graph.WAFBundleKey]*graph.WAFBundleData, len(m.bundleCache))
	maps.Copy(result, m.bundleCache)
	return result
}

// StopPoller stops the poller for a WAFGatewayBindingPolicy.
func (m *Manager) StopPoller(policyNsName types.NamespacedName) {
	m.mu.Lock()
	entry, exists := m.pollers[policyNsName]
	if !exists {
		m.mu.Unlock()
		return
	}
	delete(m.pollers, policyNsName)
	delete(m.pollErrors, policyNsName) // Clear any poll error when stopping.
	m.clearBundleCacheLocked(entry.poller)
	m.mu.Unlock()

	entry.cancel()
	m.logger.Info("Stopped WAF poller", "policy", policyNsName)
}

// stopAll stops all running pollers. Should be called during shutdown.
func (m *Manager) stopAll() {
	m.mu.Lock()
	entries := make([]*pollerEntry, 0, len(m.pollers))
	for _, entry := range m.pollers {
		entries = append(entries, entry)
	}
	m.pollers = make(map[types.NamespacedName]*pollerEntry)
	m.pollErrors = make(map[types.NamespacedName]*PollError)
	m.bundleCache = make(map[graph.WAFBundleKey]*graph.WAFBundleData)
	m.mu.Unlock()

	for _, entry := range entries {
		entry.cancel()
	}

	m.logger.Info("Stopped all WAF pollers", "count", len(entries))
}

// clearBundleCacheLocked removes cached bundle data for all bundle keys owned by the given poller.
// Must be called while m.mu is held.
func (m *Manager) clearBundleCacheLocked(p *poller) {
	for _, src := range p.getSources() {
		delete(m.bundleCache, src.BundleKey)
	}
}

// StopPollersNotIn stops all pollers whose policy namespace/name is not in the given set.
// This is used to clean up pollers for policies that have been deleted or no longer need polling.
func (m *Manager) StopPollersNotIn(activePolicies map[types.NamespacedName]struct{}) {
	m.mu.Lock()
	var toStop []types.NamespacedName
	for nsName := range m.pollers {
		if _, active := activePolicies[nsName]; !active {
			toStop = append(toStop, nsName)
		}
	}

	entriesToCancel := make([]*pollerEntry, 0, len(toStop))
	for _, nsName := range toStop {
		if entry, exists := m.pollers[nsName]; exists {
			entriesToCancel = append(entriesToCancel, entry)
			delete(m.pollers, nsName)
			delete(m.pollErrors, nsName) // Clear any poll error when stopping.
			m.clearBundleCacheLocked(entry.poller)
		}
	}
	m.mu.Unlock()

	for _, entry := range entriesToCancel {
		entry.cancel()
	}

	if len(toStop) > 0 {
		m.logger.Info("Stopped stale WAF pollers", "count", len(toStop))
	}
}
