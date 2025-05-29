package sort

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestLessObjectMeta(t *testing.T) {
	t.Parallel()
	before := metav1.Now()
	later := metav1.NewTime(before.Add(1 * time.Second))

	tests := []struct {
		meta1    *metav1.ObjectMeta
		meta2    *metav1.ObjectMeta
		name     string
		expected bool
	}{
		{
			meta1: &metav1.ObjectMeta{
				Namespace:         "ns1",
				Name:              "meta1",
				CreationTimestamp: before,
			},
			meta2: &metav1.ObjectMeta{
				Namespace:         "ns1",
				Name:              "meta2",
				CreationTimestamp: later,
			},
			name:     "first is less by timestamp",
			expected: true,
		},
		{
			meta1: &metav1.ObjectMeta{
				Namespace:         "ns1",
				Name:              "meta1",
				CreationTimestamp: before,
			},
			meta2: &metav1.ObjectMeta{
				Namespace:         "ns2",
				Name:              "meta2",
				CreationTimestamp: before,
			},
			name:     "first is less by namespace",
			expected: true,
		},
		{
			meta1: &metav1.ObjectMeta{
				Namespace:         "ns1",
				Name:              "meta1",
				CreationTimestamp: before,
			},
			meta2: &metav1.ObjectMeta{
				Namespace:         "ns1",
				Name:              "meta2",
				CreationTimestamp: before,
			},
			name:     "first is less by name",
			expected: true,
		},
		{
			meta1: &metav1.ObjectMeta{
				Namespace:         "ns1",
				Name:              "meta1",
				CreationTimestamp: before,
			},
			meta2: &metav1.ObjectMeta{
				Namespace:         "ns1",
				Name:              "meta1",
				CreationTimestamp: before,
			},
			name:     "equal",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := LessObjectMeta(test.meta1, test.meta2)
			invertedResult := LessObjectMeta(test.meta2, test.meta1)

			g.Expect(result).To(Equal(test.expected))
			g.Expect(invertedResult).To(BeFalse())
		})
	}
}

func TestLessClientObject(t *testing.T) {
	t.Parallel()
	before := metav1.Now()
	later := metav1.NewTime(before.Add(1 * time.Second))

	tests := []struct {
		obj1     client.Object
		obj2     client.Object
		name     string
		expected bool
	}{
		{
			name: "first is less by timestamp",
			obj1: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "obj1",
					CreationTimestamp: before,
				},
			},
			obj2: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "obj2",
					CreationTimestamp: later,
				},
			},
			expected: true,
		},
		{
			name: "first is less by namespace",
			obj1: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "obj1",
					CreationTimestamp: before,
				},
			},
			obj2: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns2",
					Name:              "obj1",
					CreationTimestamp: before,
				},
			},
			expected: true,
		},
		{
			name: "first is less by name",
			obj1: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "obj1",
					CreationTimestamp: before,
				},
			},
			obj2: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "obj2",
					CreationTimestamp: before,
				},
			},
			expected: true,
		},
		{
			name: "equal",
			obj1: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "obj1",
					CreationTimestamp: before,
				},
			},
			obj2: &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "ns1",
					Name:              "obj1",
					CreationTimestamp: before,
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			result := LessClientObject(test.obj1, test.obj2)
			invertedResult := LessClientObject(test.obj2, test.obj1)

			g.Expect(result).To(Equal(test.expected))
			g.Expect(invertedResult).To(BeFalse())
		})
	}
}
