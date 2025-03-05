package predicate

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SecretNamePredicate implements a predicate function that returns true if the Secret matches the expected
// namespace and one of the expected names.
type SecretNamePredicate struct {
	predicate.Funcs
	Namespace   string
	SecretNames []string
}

// Create filters CreateEvents based on the Secret name.
func (sp SecretNamePredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}

	if secret, ok := e.Object.(*corev1.Secret); ok {
		return secretMatches(secret, sp.Namespace, sp.SecretNames)
	}

	return false
}

// Update filters UpdateEvents based on the Secret name.
func (sp SecretNamePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectNew == nil {
		return false
	}

	if secret, ok := e.ObjectNew.(*corev1.Secret); ok {
		return secretMatches(secret, sp.Namespace, sp.SecretNames)
	}

	return false
}

// Delete filters DeleteEvents based on the Secret name.
func (sp SecretNamePredicate) Delete(e event.DeleteEvent) bool {
	if e.Object == nil {
		return false
	}

	if secret, ok := e.Object.(*corev1.Secret); ok {
		return secretMatches(secret, sp.Namespace, sp.SecretNames)
	}

	return false
}

// Generic filters GenericEvents based on the Secret name.
func (sp SecretNamePredicate) Generic(e event.GenericEvent) bool {
	if e.Object == nil {
		return false
	}

	if secret, ok := e.Object.(*corev1.Secret); ok {
		return secretMatches(secret, sp.Namespace, sp.SecretNames)
	}

	return false
}

func secretMatches(secret *corev1.Secret, namespace string, names []string) bool {
	if secret.GetNamespace() != namespace {
		return false
	}

	return slices.Contains(names, secret.GetName())
}
