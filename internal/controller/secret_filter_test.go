package controller

import (
	"testing"

	. "github.com/onsi/gomega"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sevent "sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/config"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/events"
)

func newTestSecret(namespace, name string, secretLabels map[string]string) *apiv1.Secret {
	return &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    secretLabels,
		},
	}
}

func newTestFilter(t *testing.T, cfg config.Config) *secretLabelFilter {
	t.Helper()
	g := NewWithT(t)
	filter, err := newSecretLabelFilter(cfg)
	g.Expect(err).ToNot(HaveOccurred())
	return filter
}

func TestNewSecretLabelFilter(t *testing.T) {
	t.Parallel()

	t.Run("no selector allows everything", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		filter := newTestFilter(t, config.Config{
			GatewayPodConfig: config.GatewayPodConfig{Namespace: "ngf-ns"},
		})

		g.Expect(filter.allows(newTestSecret("ngf-ns", "unlabeled", nil))).To(BeTrue())
	})

	t.Run("invalid selector returns error", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		_, err := newSecretLabelFilter(config.Config{SecretLabelSelector: "!!!invalid"})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("unsatisfiable selector returns error", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		_, err := newSecretLabelFilter(config.Config{SecretLabelSelector: "a=1,a!=1"})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("derived secret labels match the selector", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		filter := newTestFilter(t, config.Config{
			SecretLabelSelector: "gateway.nginx.org/watch=true,tier>2",
		})
		g.Expect(filter.derivedSecretLabels()).To(Equal(map[string]string{
			"gateway.nginx.org/watch": "true",
			"tier":                    "3",
		}))

		noSelector := newTestFilter(t, config.Config{})
		g.Expect(noSelector.derivedSecretLabels()).To(BeNil())
	})
}

func TestSecretLabelFilterAllows(t *testing.T) {
	t.Parallel()

	const (
		licenseName = "nplus-license"
		plmName     = "plm-creds"
		plmCAName   = "other-ns/plm-ca"
	)

	cfg := config.Config{
		SecretLabelSelector: "gateway.nginx.org/watch=true",
		Plus:                true,
		UsageReportConfig: config.UsageReportConfig{
			SecretName: licenseName,
		},
		PLMStorageConfig: &config.PLMStorageConfig{
			URL:                   "https://plm.example.com",
			CredentialsSecretName: plmName,
			CASecretName:          plmCAName,
		},
		GatewayPodConfig: config.GatewayPodConfig{Namespace: "ngf-ns"},
	}

	tests := []struct {
		secret   *apiv1.Secret
		name     string
		expected bool
	}{
		{
			name:     "labeled secret in NGF namespace",
			secret:   newTestSecret("ngf-ns", "cert", map[string]string{"gateway.nginx.org/watch": "true"}),
			expected: true,
		},
		{
			name:     "unlabeled secret in NGF namespace",
			secret:   newTestSecret("ngf-ns", "cert", nil),
			expected: false,
		},
		{
			name:     "secret in NGF namespace with wrong label value",
			secret:   newTestSecret("ngf-ns", "cert", map[string]string{"gateway.nginx.org/watch": "false"}),
			expected: false,
		},
		{
			name:     "unlabeled secret in another namespace",
			secret:   newTestSecret("app-ns", "cert", nil),
			expected: true,
		},
		{
			name:     "Plus license secret in NGF namespace",
			secret:   newTestSecret("ngf-ns", "nplus-license", nil),
			expected: true,
		},
		{
			name:     "PLM credentials secret in NGF namespace",
			secret:   newTestSecret("ngf-ns", "plm-creds", nil),
			expected: true,
		},
		{
			name:     "PLM CA secret in another namespace",
			secret:   newTestSecret("other-ns", "plm-ca", nil),
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			filter := newTestFilter(t, cfg)
			g.Expect(filter.allows(test.secret)).To(Equal(test.expected))
		})
	}
}

func TestSecretLabelFilterReferenceFilter(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		SecretLabelSelector: "gateway.nginx.org/watch=true",
		Plus:                true,
		UsageReportConfig: config.UsageReportConfig{
			SecretName: "nplus-license",
		},
		GatewayPodConfig: config.GatewayPodConfig{Namespace: "ngf-ns"},
	}

	tests := []struct {
		secret   *apiv1.Secret
		name     string
		expected bool
	}{
		{
			name:     "labeled secret in NGF namespace",
			secret:   newTestSecret("ngf-ns", "cert", map[string]string{"gateway.nginx.org/watch": "true"}),
			expected: true,
		},
		{
			name:     "unlabeled secret in NGF namespace",
			secret:   newTestSecret("ngf-ns", "cert", nil),
			expected: false,
		},
		{
			name:     "allowlisted Plus license secret in NGF namespace stays non-referenceable",
			secret:   newTestSecret("ngf-ns", "nplus-license", nil),
			expected: false,
		},
		{
			name:     "unlabeled secret in another namespace",
			secret:   newTestSecret("app-ns", "cert", nil),
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			filter := newTestFilter(t, cfg)
			g.Expect(filter.referenceFilter()(test.secret)).To(Equal(test.expected))
		})
	}

	g := NewWithT(t)

	noSelector := newTestFilter(t, config.Config{GatewayPodConfig: config.GatewayPodConfig{Namespace: "ngf-ns"}})
	g.Expect(noSelector.referenceFilter()).To(BeNil())

	var nilFilter *secretLabelFilter
	g.Expect(nilFilter.referenceFilter()).To(BeNil())
}

func TestSecretLabelFilterPredicate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := config.Config{
		SecretLabelSelector: "gateway.nginx.org/watch=true",
		GatewayPodConfig:    config.GatewayPodConfig{Namespace: "ngf-ns"},
	}
	filter := newTestFilter(t, cfg)
	pred := filter.predicate()

	labeled := newTestSecret("ngf-ns", "labeled", map[string]string{"gateway.nginx.org/watch": "true"})
	unlabeled := newTestSecret("ngf-ns", "unlabeled", nil)
	other := newTestSecret("app-ns", "unlabeled", nil)

	g.Expect(pred.Create(k8sevent.CreateEvent{Object: labeled})).To(BeTrue())
	g.Expect(pred.Create(k8sevent.CreateEvent{Object: unlabeled})).To(BeFalse())
	g.Expect(pred.Create(k8sevent.CreateEvent{Object: other})).To(BeTrue())

	g.Expect(pred.Delete(k8sevent.DeleteEvent{Object: unlabeled})).To(BeFalse())
	g.Expect(pred.Generic(k8sevent.GenericEvent{Object: unlabeled})).To(BeFalse())

	// Label removal must pass so the Secret is dropped from the graph.
	g.Expect(pred.Update(k8sevent.UpdateEvent{ObjectOld: labeled, ObjectNew: unlabeled})).To(BeTrue())
	// Label addition must pass so the Secret enters the graph.
	g.Expect(pred.Update(k8sevent.UpdateEvent{ObjectOld: unlabeled, ObjectNew: labeled})).To(BeTrue())
	// Never-allowed Secret updates stay dropped.
	g.Expect(pred.Update(k8sevent.UpdateEvent{ObjectOld: unlabeled, ObjectNew: unlabeled})).To(BeFalse())
}

func TestSecretLabelFilterEvent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cfg := config.Config{
		SecretLabelSelector: "gateway.nginx.org/watch=true",
		GatewayPodConfig:    config.GatewayPodConfig{Namespace: "ngf-ns"},
	}
	filter := newTestFilter(t, cfg)

	unlabeled := newTestSecret("ngf-ns", "cert", nil)
	labeled := newTestSecret("ngf-ns", "cert", map[string]string{"gateway.nginx.org/watch": "true"})
	other := newTestSecret("app-ns", "cert", nil)

	// Disallowed Secret upserts become deletes.
	out := filter.filterEvent(&events.UpsertEvent{Resource: unlabeled})
	del, ok := out.(*events.DeleteEvent)
	g.Expect(ok).To(BeTrue())
	g.Expect(del.NamespacedName).To(Equal(types.NamespacedName{Namespace: "ngf-ns", Name: "cert"}))
	g.Expect(del.Type).To(BeAssignableToTypeOf(&apiv1.Secret{}))

	// Allowed Secret upserts pass through.
	upsert := &events.UpsertEvent{Resource: labeled}
	g.Expect(filter.filterEvent(upsert)).To(BeIdenticalTo(upsert))
	g.Expect(filter.filterEvent(&events.UpsertEvent{Resource: other})).To(BeAssignableToTypeOf(&events.UpsertEvent{}))

	// Non-Secret events pass through.
	nonSecret := &events.UpsertEvent{Resource: &apiv1.ConfigMap{}}
	g.Expect(filter.filterEvent(nonSecret)).To(BeIdenticalTo(nonSecret))

	// Delete events pass through unchanged.
	delEvent := &events.DeleteEvent{
		Type:           &apiv1.Secret{},
		NamespacedName: types.NamespacedName{Namespace: "ngf-ns", Name: "cert"},
	}
	g.Expect(filter.filterEvent(delEvent)).To(BeIdenticalTo(delEvent))

	// Nil filter passes through.
	var nilFilter *secretLabelFilter
	g.Expect(nilFilter.filterEvent(upsert)).To(BeIdenticalTo(upsert))
	g.Expect(nilFilter.allows(unlabeled)).To(BeTrue())
}
