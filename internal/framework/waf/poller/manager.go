package poller

import (
	"context"
	"maps"
	"sync"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/agent"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/waf/fetch"
)

//go:generate go tool counterfeiter -generate

// PollError represents a polling error for a specific bundle.
type PollError struct {
	Err error
	// BundleKey is the internal identifier of the bundle that failed.
	BundleKey graph.WAFBundleKey
	// BundleDescription is a human-readable label for the bundle,
	// e.g. "policy bundle" or "security log bundle (profile: default)".
	BundleDescription string
}

// BundleUpdate records the most recent poll cycle in which a changed bundle was detected and
// dispatched to target deployments. It does not confirm that any deployment applied the update.
type BundleUpdate struct {
	UpdatedAt metav1.Time
	// BundleKey is the internal identifier of the bundle that was updated.
	BundleKey graph.WAFBundleKey
	// BundleDescription is a human-readable label for the bundle,
	// e.g. "policy bundle" or "security log bundle (profile: default)".
	BundleDescription string
	Checksum          string
}

// Manager is the interface for managing WAF bundle pollers.
//
//counterfeiter:generate . Manager
type Manager interface {
	// ReconcilePoller ensures a poller is running with the correct configuration.
	ReconcilePoller(ctx context.Context, cfg Config)
	// GetAllPollErrors returns a deep copy of all current poll errors.
	GetAllPollErrors() map[types.NamespacedName]PollError
	// GetAllBundleUpdates returns a copy of the most recent bundle change detected per policy.
	GetAllBundleUpdates() map[types.NamespacedName]BundleUpdate
	// GetLatestBundles returns a copy of all bundles that have been successfully fetched by pollers.
	// These represent the freshest known bundle data and should take precedence over
	// graph-cached bundles when constructing stale-bundle fallback state.
	GetLatestBundles() map[graph.WAFBundleKey]*graph.WAFBundleData
	// StopPoller stops the poller for a WAFPolicy.
	StopPoller(policyNsName types.NamespacedName)
	// StopPollersNotIn stops all pollers whose policy namespace/name is not in the given set.
	StopPollersNotIn(activePolicies map[types.NamespacedName]struct{})
	// HasPoller reports whether a poller is currently registered for the given policy.
	HasPoller(policyNsName types.NamespacedName) bool
}

// pollerManager manages the lifecycle of all WAF bundle pollers.
// It creates, tracks, and stops pollers as WAFPolicies are created, updated, or deleted.
type pollerManager struct {
	fetcher       fetch.Fetcher
	deployments   agent.DeploymentStorer
	pollers       map[types.NamespacedName]*pollerEntry
	pollErrors    map[types.NamespacedName]*PollError
	bundleUpdates map[types.NamespacedName]BundleUpdate
	bundleCache   map[graph.WAFBundleKey]*graph.WAFBundleData
	// bundleKeyToPolicy maps each bundle key to the policy that owns it.
	// Used to look up the policy namespace/name when injecting a WAFBundleReconcileEvent.
	bundleKeyToPolicy map[graph.WAFBundleKey]types.NamespacedName
	// bundleKeyToDescription maps each bundle key to a human-readable description of the bundle,
	// e.g. "policy bundle" or "security log bundle (profile: default)".
	// Used to produce user-visible condition messages without exposing internal key formats.
	bundleKeyToDescription map[graph.WAFBundleKey]string
	statusCallback         func(targets []types.NamespacedName)
	// eventCh is the send side of the main event loop channel.
	// A WAFBundleReconcileEvent is sent when a previously-pending bundle is first fetched successfully,
	// triggering an immediate re-reconcile so the Gateway config push can proceed.
	eventCh chan<- any
	// ctx is the root context for the manager, used to cancel goroutines on shutdown.
	ctx    context.Context
	logger logr.Logger
	mu     sync.RWMutex
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
	EventCh        chan<- any
	// Ctx is the root context for the manager lifetime.
	// It is used to cancel goroutines that inject events into the event loop on shutdown.
	Ctx    context.Context
	Logger logr.Logger
}

// NewManager creates a new Manager.
// It panics if EventCh is set without Ctx, as the event-injection goroutine requires
// a context to avoid leaking on shutdown.
func NewManager(cfg ManagerConfig) Manager {
	if cfg.EventCh != nil && cfg.Ctx == nil {
		panic("waf.ManagerConfig: Ctx must be set when EventCh is set")
	}
	return &pollerManager{
		logger:                 cfg.Logger,
		fetcher:                cfg.Fetcher,
		deployments:            cfg.Deployments,
		pollers:                make(map[types.NamespacedName]*pollerEntry),
		pollErrors:             make(map[types.NamespacedName]*PollError),
		bundleUpdates:          make(map[types.NamespacedName]BundleUpdate),
		bundleCache:            make(map[graph.WAFBundleKey]*graph.WAFBundleData),
		bundleKeyToPolicy:      make(map[graph.WAFBundleKey]types.NamespacedName),
		bundleKeyToDescription: make(map[graph.WAFBundleKey]string),
		statusCallback:         cfg.StatusCallback,
		eventCh:                cfg.EventCh,
		ctx:                    cfg.Ctx,
	}
}

// Config contains configuration for reconciling a poller.
type Config struct {
	InitialChecksums  map[graph.WAFBundleKey]string
	PolicyNsName      types.NamespacedName
	Sources           []BundleSource
	TargetDeployments []types.NamespacedName
}

// ReconcilePoller ensures a poller is running with the correct configuration.
// If no poller exists, one is started. If a poller exists with the same sources,
// only the target deployments are updated. If sources have changed, the poller is restarted.
// This avoids unnecessary poller restarts when only targets change.
func (m *pollerManager) ReconcilePoller(ctx context.Context, cfg Config) {
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
func (m *pollerManager) startPoller(ctx context.Context, cfg Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Always cancel any existing poller before overwriting the map entry.
	// This is safe even if the caller didn't observe one, because another
	// goroutine may have started one between our check and acquiring this lock.
	if entry, exists := m.pollers[cfg.PolicyNsName]; exists {
		m.logger.V(1).Info("Stopping existing poller before starting new one", "policy", cfg.PolicyNsName)
		entry.cancel()
		delete(m.pollErrors, cfg.PolicyNsName)
		delete(m.bundleUpdates, cfg.PolicyNsName)
		m.clearBundleCacheLocked(entry.poller)
	}

	pollerCtx, cancel := context.WithCancel(ctx) //nolint:gosec // Cancel is handled externally to this function

	var poller *poller

	// Create a wrapped callback that records poll results and triggers a status update
	// scoped to just the poller's target deployments.
	wrappedCallback := func(
		policyNsName types.NamespacedName,
		bundleKey graph.WAFBundleKey,
		newChecksum string,
		err error,
	) {
		m.mu.RLock()
		desc := m.bundleKeyToDescription[bundleKey]
		m.mu.RUnlock()
		if desc == "" {
			desc = "WAF bundle"
		}
		m.recordPollResult(policyNsName, bundleKey, desc, newChecksum, err)
		if m.statusCallback != nil {
			m.statusCallback(poller.getTargetDeployments())
		}
	}

	// Record which policy owns each bundle key and its human-readable description.
	for _, src := range cfg.Sources {
		m.bundleKeyToPolicy[src.BundleKey] = cfg.PolicyNsName
		desc := src.Description
		if desc == "" {
			desc = "WAF bundle"
		}
		m.bundleKeyToDescription[src.BundleKey] = desc
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
// If err is nil, it clears any previous error for the same bundle key.
// If newChecksum is non-empty, the bundle was successfully updated and the update is recorded.
// If err is non-nil, it stores the error for this bundle key.
// This method is called by the internal status callback.
func (m *pollerManager) recordPollResult(
	policyNsName types.NamespacedName,
	bundleKey graph.WAFBundleKey,
	bundleDescription string,
	newChecksum string,
	err error,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err == nil {
		if existing := m.pollErrors[policyNsName]; existing != nil && existing.BundleKey == bundleKey {
			delete(m.pollErrors, policyNsName)
		}
		if newChecksum != "" {
			m.bundleUpdates[policyNsName] = BundleUpdate{
				BundleKey:         bundleKey,
				BundleDescription: bundleDescription,
				Checksum:          newChecksum,
				UpdatedAt:         metav1.Now(),
			}
		}
	} else {
		m.pollErrors[policyNsName] = &PollError{
			BundleKey:         bundleKey,
			BundleDescription: bundleDescription,
			Err:               err,
		}
	}
}

// GetAllBundleUpdates returns a copy of the most recent successful bundle update per policy.
func (m *pollerManager) GetAllBundleUpdates() map[types.NamespacedName]BundleUpdate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.bundleUpdates) == 0 {
		return nil
	}

	result := make(map[types.NamespacedName]BundleUpdate, len(m.bundleUpdates))
	maps.Copy(result, m.bundleUpdates)
	return result
}

// cacheBundleUpdate stores the latest successfully polled bundle data in the manager's cache.
// This is called by pollers when they detect a changed bundle, ensuring the freshest data
// is available for graph rebuild stale-bundle fallback.
// On the first time a bundle key appears in this manager's cache, a WAFBundleReconcileEvent
// is injected into the event loop to trigger an immediate graph rebuild.
// Note: this fires on any first-cache event, including after a poller restart that cleared the
// cache — not only when the policy was previously in BundlePending state. A spurious reconcile
// event in that case is harmless: it triggers an unnecessary graph rebuild but causes no
// incorrect behavior.
func (m *pollerManager) cacheBundleUpdate(bundleKey graph.WAFBundleKey, data []byte, checksum string) {
	m.mu.Lock()

	_, alreadyCached := m.bundleCache[bundleKey]

	m.bundleCache[bundleKey] = &graph.WAFBundleData{
		Data:     data,
		Checksum: checksum,
	}

	// Capture event details while holding the lock, then release before sending.
	var event *events.WAFBundleReconcileEvent
	if !alreadyCached {
		if policyNsName, ok := m.bundleKeyToPolicy[bundleKey]; ok && m.eventCh != nil {
			event = &events.WAFBundleReconcileEvent{PolicyNsName: policyNsName}
		}
	}

	m.mu.Unlock()

	// Send the reconcile event after releasing the lock so other manager operations are not
	// blocked on the mutex while waiting for the event loop. The manager's root context is
	// used as a cancellation escape hatch: on shutdown, the event loop exits before the
	// manager's context is canceled, so without this the poller goroutine could block
	// indefinitely trying to send to an already-drained channel.
	if event != nil {
		select {
		case m.eventCh <- *event:
		case <-m.ctx.Done():
		}
	}
}

// GetAllPollErrors returns a deep copy of all current poll errors.
func (m *pollerManager) GetAllPollErrors() map[types.NamespacedName]PollError {
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
func (m *pollerManager) GetLatestBundles() map[graph.WAFBundleKey]*graph.WAFBundleData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.bundleCache) == 0 {
		return nil
	}

	result := make(map[graph.WAFBundleKey]*graph.WAFBundleData, len(m.bundleCache))
	maps.Copy(result, m.bundleCache)
	return result
}

// StopPoller stops the poller for a WAFPolicy.
func (m *pollerManager) StopPoller(policyNsName types.NamespacedName) {
	m.mu.Lock()
	entry, exists := m.pollers[policyNsName]
	if !exists {
		m.mu.Unlock()
		return
	}
	delete(m.pollers, policyNsName)
	delete(m.pollErrors, policyNsName)
	delete(m.bundleUpdates, policyNsName)
	m.clearBundleCacheLocked(entry.poller)
	m.mu.Unlock()

	entry.cancel()
	m.logger.Info("Stopped WAF poller", "policy", policyNsName)
}

// stopAll stops all running pollers. Should be called during shutdown.
func (m *pollerManager) stopAll() {
	m.mu.Lock()
	entries := make([]*pollerEntry, 0, len(m.pollers))
	for _, entry := range m.pollers {
		entries = append(entries, entry)
	}
	m.pollers = make(map[types.NamespacedName]*pollerEntry)
	m.pollErrors = make(map[types.NamespacedName]*PollError)
	m.bundleUpdates = make(map[types.NamespacedName]BundleUpdate)
	m.bundleCache = make(map[graph.WAFBundleKey]*graph.WAFBundleData)
	m.bundleKeyToPolicy = make(map[graph.WAFBundleKey]types.NamespacedName)
	m.bundleKeyToDescription = make(map[graph.WAFBundleKey]string)
	m.mu.Unlock()

	for _, entry := range entries {
		entry.cancel()
	}

	m.logger.Info("Stopped all WAF pollers", "count", len(entries))
}

// clearBundleCacheLocked removes cached bundle data and all per-key mappings for all bundle keys
// owned by the given poller. Must be called while m.mu is held.
func (m *pollerManager) clearBundleCacheLocked(p *poller) {
	for _, src := range p.getSources() {
		delete(m.bundleCache, src.BundleKey)
		delete(m.bundleKeyToPolicy, src.BundleKey)
		delete(m.bundleKeyToDescription, src.BundleKey)
	}
}

// HasPoller reports whether a poller is currently registered for the given policy.
func (m *pollerManager) HasPoller(policyNsName types.NamespacedName) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.pollers[policyNsName]
	return exists
}

// StopPollersNotIn stops all pollers whose policy namespace/name is not in the given set.
// This is used to clean up pollers for policies that have been deleted or no longer need polling.
func (m *pollerManager) StopPollersNotIn(activePolicies map[types.NamespacedName]struct{}) {
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
			delete(m.pollErrors, nsName)
			delete(m.bundleUpdates, nsName)
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
