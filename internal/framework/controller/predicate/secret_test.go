package predicate

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestSecretNamePredicate(t *testing.T) {
	t.Parallel()

	pred := SecretNamePredicate{
		Namespace:   "test-namespace",
		SecretNames: []string{"secret1", "secret2"},
	}

	tests := []struct {
		createEvent  *event.CreateEvent
		updateEvent  *event.UpdateEvent
		deleteEvent  *event.DeleteEvent
		genericEvent *event.GenericEvent
		name         string
		expUpdate    bool
	}{
		{
			name: "Create event with matching secret",
			createEvent: &event.CreateEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "Create event with non-matching secret",
			createEvent: &event.CreateEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret3",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "Create event with non-matching namespace",
			createEvent: &event.CreateEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "other-namespace",
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "Update event with matching secret",
			updateEvent: &event.UpdateEvent{
				ObjectNew: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret2",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "Update event with non-matching secret",
			updateEvent: &event.UpdateEvent{
				ObjectNew: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret3",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "Update event with non-matching namespace",
			updateEvent: &event.UpdateEvent{
				ObjectNew: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "other-namespace",
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "Delete event with matching secret",
			deleteEvent: &event.DeleteEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "Delete event with non-matching secret",
			deleteEvent: &event.DeleteEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret3",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "Delete event with non-matching namespace",
			deleteEvent: &event.DeleteEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "other-namespace",
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "Generic event with matching secret",
			genericEvent: &event.GenericEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: true,
		},
		{
			name: "Generic event with non-matching secret",
			genericEvent: &event.GenericEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret3",
						Namespace: "test-namespace",
					},
				},
			},
			expUpdate: false,
		},
		{
			name: "Generic event with non-matching namespace",
			genericEvent: &event.GenericEvent{
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret1",
						Namespace: "other-namespace",
					},
				},
			},
			expUpdate: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			var result bool
			switch {
			case test.createEvent != nil:
				result = pred.Create(*test.createEvent)
			case test.updateEvent != nil:
				result = pred.Update(*test.updateEvent)
			case test.deleteEvent != nil:
				result = pred.Delete(*test.deleteEvent)
			default:
				result = pred.Generic(*test.genericEvent)
			}

			g.Expect(test.expUpdate).To(Equal(result))
		})
	}
}
