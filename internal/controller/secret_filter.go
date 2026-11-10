package controller

import (
	"fmt"
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	k8spredicate "sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

// secretLabelFilter gates which Secrets in the NGF namespace can be used by Gateway resources when
// --watch-secret-label-selector is set. NGF's own Secrets live in the NGF namespace and must stay
// cached unlabeled so the provisioner can copy them, so the cache cannot filter them. Instead, any
// Secret in the NGF namespace that does not match the selector and is not required by NGF itself is
// excluded from the state graph, making it invisible to Gateway resources just like in other namespaces.
type secretLabelFilter struct {
	// selector is the label selector configured via --watch-secret-label-selector.
	selector labels.Selector
	// secretLabels contains the labels NGF applies to the Secrets it copies into Gateway namespaces.
	secretLabels map[string]string
	// allowed contains the Secrets explicitly allowed regardless of their labels.
	allowed map[types.NamespacedName]struct{}
	// ngfNamespace is the namespace where NGF's own Secrets live.
	ngfNamespace string
}

// newSecretLabelFilter creates the filter from the configuration. If no selector is set, the returned
// filter allows all Secrets.
func newSecretLabelFilter(cfg config.Config) (*secretLabelFilter, error) {
	selector := labels.Everything()
	var secretLabels map[string]string
	if cfg.SecretLabelSelector != "" {
		parsed, err := labels.Parse(cfg.SecretLabelSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid secret label selector %q: %w", cfg.SecretLabelSelector, err)
		}
		secretLabels, err = secretLabelsForSelector(parsed)
		if err != nil {
			return nil, err
		}
		selector = parsed
	}

	allowed := make(map[types.NamespacedName]struct{})
	if cfg.Plus {
		namespace := cfg.GatewayPodConfig.Namespace
		for _, name := range []string{
			cfg.UsageReportConfig.SecretName,
			cfg.UsageReportConfig.CASecretName,
			cfg.UsageReportConfig.ClientSSLSecretName,
		} {
			if name != "" {
				allowed[types.NamespacedName{Namespace: namespace, Name: name}] = struct{}{}
			}
		}
	}
	for nsname := range buildPLMSecretNames(cfg.PLMStorageConfig, cfg.GatewayPodConfig.Namespace) {
		allowed[nsname] = struct{}{}
	}

	return &secretLabelFilter{
		ngfNamespace: cfg.GatewayPodConfig.Namespace,
		selector:     selector,
		secretLabels: secretLabels,
		allowed:      allowed,
	}, nil
}

// derivedSecretLabels returns the labels NGF applies to the Secrets it copies into Gateway namespaces
// so they stay visible to the label-filtered cache. It is nil when no selector is configured.
func (f *secretLabelFilter) derivedSecretLabels() map[string]string {
	if f == nil {
		return nil
	}
	return f.secretLabels
}

// secretSelector returns the selector configured via --watch-secret-label-selector, or nil when no
// selector is configured. The provisioner uses it to keep copied Secrets cache-visible.
func (f *secretLabelFilter) secretSelector() labels.Selector {
	if f == nil || f.secretLabels == nil {
		return nil
	}
	return f.selector
}

// allows returns true if the Secret may be used by Gateway resources.
func (f *secretLabelFilter) allows(secret *apiv1.Secret) bool {
	if f == nil || secret.Namespace != f.ngfNamespace {
		return true
	}
	if _, ok := f.allowed[client.ObjectKeyFromObject(secret)]; ok {
		return true
	}
	return f.selector.Matches(labels.Set(secret.Labels))
}

// filterEvent converts an UpsertEvent for a Secret that is not allowed into a DeleteEvent, so the
// state graph treats the Secret as absent. A Secret whose label is removed also arrives here as an
// update (the predicate lets updates through when the old version was allowed), and the resulting
// DeleteEvent removes it from the graph.
func (f *secretLabelFilter) filterEvent(evt any) any {
	if f == nil {
		return evt
	}

	upsert, ok := evt.(*events.UpsertEvent)
	if !ok {
		return evt
	}

	secret, ok := upsert.Resource.(*apiv1.Secret)
	if !ok || f.allows(secret) {
		return evt
	}

	return &events.DeleteEvent{
		Type:           &apiv1.Secret{},
		NamespacedName: client.ObjectKeyFromObject(secret),
	}
}

// predicate returns a controller-runtime predicate that drops disallowed Secrets before they reach
// the reconciler, so they do not even appear in logs. Updates pass when either the old or the new
// version is allowed, so removing a label still produces an event that filterEvent turns into a delete.
func (f *secretLabelFilter) predicate() k8spredicate.Predicate {
	allowed := func(obj client.Object) bool {
		secret, ok := obj.(*apiv1.Secret)
		return !ok || f.allows(secret)
	}

	return k8spredicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return allowed(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return allowed(e.ObjectOld) || allowed(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return allowed(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return allowed(e.Object)
		},
	}
}

// ngfOwnedSecretLabelKeys are labels NGF applies to the resources it provisions.
var ngfOwnedSecretLabelKeys = map[string]struct{}{
	controller.GatewayLabel:      {},
	controller.AppNameLabel:      {},
	controller.AppInstanceLabel:  {},
	controller.AppManagedByLabel: {},
}

// secretLabelsForSelector derives a label set from the selector that the selector itself matches.
// NGF applies these labels to the Secrets it copies into Gateway namespaces so that they remain
// visible under the selector.
func secretLabelsForSelector(selector labels.Selector) (map[string]string, error) {
	requirements, selectable := selector.Requirements()
	if !selectable {
		return nil, fmt.Errorf("secret label selector %q can never be satisfied", selector)
	}

	secretLabels := make(map[string]string)
	for _, req := range requirements {
		key := req.Key()
		if _, ok := ngfOwnedSecretLabelKeys[key]; ok {
			return nil, fmt.Errorf("secret label selector %q must not select on NGF-owned label %q", selector, key)
		}
		switch req.Operator() {
		case selection.Equals, selection.DoubleEquals, selection.In:
			vals := req.Values().List()
			if len(vals) == 0 {
				continue
			}
			secretLabels[key] = satisfyingValue(key, vals, requirements)
		case selection.Exists:
			if _, ok := secretLabels[key]; !ok {
				secretLabels[key] = ""
			}
		case selection.GreaterThan, selection.LessThan:
			vals := req.Values().List()
			if len(vals) == 0 {
				continue
			}
			num, err := strconv.ParseInt(vals[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf(
					"secret label selector %q has non-numeric value %q for key %q: %w", selector, vals[0], key, err,
				)
			}
			delta := int64(1)
			if req.Operator() == selection.LessThan {
				delta = -1
			}
			secretLabels[key] = strconv.FormatInt(num+delta, 10)
		}
	}

	if !selector.Matches(labels.Set(secretLabels)) {
		return nil, fmt.Errorf("secret label selector %q can never be satisfied by a labeled Secret", selector)
	}

	return secretLabels, nil
}

// satisfyingValue returns the first candidate value that matches every requirement on key (for
// example env in (dev,prod) together with env!=dev yields prod), falling back to the first candidate
// when none matches so the caller's final check reports the unsatisfiable selector.
func satisfyingValue(key string, candidates []string, requirements []labels.Requirement) string {
	for _, value := range candidates {
		matches := true
		for _, req := range requirements {
			if req.Key() == key && !req.Matches(labels.Set{key: value}) {
				matches = false
				break
			}
		}
		if matches {
			return value
		}
	}
	return candidates[0]
}
