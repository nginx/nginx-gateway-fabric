package index

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPodIPIndexFunc(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		msg       string
		obj       client.Object
		expOutput []string
	}{
		{
			msg: "normal case",
			obj: &corev1.Pod{
				Status: corev1.PodStatus{
					PodIP: "1.2.3.4",
				},
			},
			expOutput: []string{"1.2.3.4"},
		},
		{
			msg:       "empty status",
			obj:       &corev1.Pod{},
			expOutput: []string{""},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			output := PodIPIndexFunc(tc.obj)
			g.Expect(output).To(Equal(tc.expOutput))
		})
	}
}

func TestPodIPIndexFuncPanics(t *testing.T) {
	t.Parallel()
	defer func() {
		g := NewWithT(t)
		g.Expect(recover()).ToNot(BeNil())
	}()

	PodIPIndexFunc(&corev1.Namespace{})
}
