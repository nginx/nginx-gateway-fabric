package telemetry_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/telemetry"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kubernetes/kubernetesfakes"
)

func TestCollect_Success(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ngfPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ngf-pod",
			Namespace: "nginx-gateway",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "replicaset1",
				},
			},
		},
	}

	replicas := int32(1)
	ngfReplicaSet := &appsv1.ReplicaSet{
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "replica",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "ngf-deployment",
					UID:  "test-uid-replicaSet",
				},
			},
		},
	}

	kubeNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: metav1.NamespaceSystem,
			UID:  "test-uid",
		},
	}

	k8sClientReader := &kubernetesfakes.FakeReader{}

	cfg := telemetry.LabelCollectorConfig{
		K8sClientReader: k8sClientReader,
		Version:         "my-version",
		PodNSName: types.NamespacedName{
			Name:      "ngf-pod",
			Namespace: "nginx-gateway",
		},
	}

	baseGetCalls := createGetCallsFunc(ngfPod, ngfReplicaSet, kubeNamespace)
	k8sClientReader.GetCalls(baseGetCalls)

	c := telemetry.NewLabelCollector(cfg)
	labels, err := c.Collect(t.Context())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(labels).To(Equal(map[string]string{
		"product-type":      "ngf",
		"product-version":   "my-version",
		"cluster-id":        "test-uid",
		"control-name":      "ngf-deployment",
		"control-namespace": "nginx-gateway",
		"control-id":        "test-uid-replicaSet",
	}))
}

func TestCollect_Errors(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	ngfPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ngf-pod",
			Namespace: "nginx-gateway",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "replicaset1",
				},
			},
		},
	}

	replicas := int32(1)
	ngfReplicaSet := &appsv1.ReplicaSet{
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "replica",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "Deployment1",
					UID:  "test-uid-replicaSet",
				},
			},
		},
	}

	kubeNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: metav1.NamespaceSystem,
			UID:  "test-uid",
		},
	}

	baseGetCalls := createGetCallsFunc(ngfPod, ngfReplicaSet, kubeNamespace)

	mergeGetCallsWithBase := func(f getCallsFunc) getCallsFunc {
		return func(
			ctx context.Context,
			nsName types.NamespacedName,
			object client.Object,
			option ...client.GetOption,
		) error {
			err := baseGetCalls(ctx, nsName, object, option...)
			g.Expect(err).ToNot(HaveOccurred())

			return f(ctx, nsName, object, option...)
		}
	}

	tests := []struct {
		name           string
		getCallsFunc   getCallsFunc
		wantErrContain string
	}{
		{
			name: "collectClusterID error",
			getCallsFunc: mergeGetCallsWithBase(func(
				_ context.Context,
				_ types.NamespacedName,
				object client.Object,
				_ ...client.GetOption,
			) error {
				if _, ok := object.(*v1.Namespace); ok {
					return errors.New("clusterID fail")
				}
				return nil
			}),
			wantErrContain: "failed to collect cluster information",
		},
		{
			name: "getPodReplicaSet error",
			getCallsFunc: mergeGetCallsWithBase(func(
				_ context.Context,
				_ types.NamespacedName,
				object client.Object,
				_ ...client.GetOption,
			) error {
				if _, ok := object.(*appsv1.ReplicaSet); ok {
					return errors.New("replicaSet fail")
				}
				return nil
			}),
			wantErrContain: "failed to get replica set for pod",
		},
		{
			name: "getDeploymentID error",
			getCallsFunc: mergeGetCallsWithBase(createGetCallsFunc(&appsv1.ReplicaSet{
				Spec: appsv1.ReplicaSetSpec{
					Replicas: &replicas,
				},
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "replica",
							Kind: "Deployment",
						},
					},
				},
			})),
			wantErrContain: "failed to get NGF deploymentID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			getCalls := tt.getCallsFunc

			k8sClientReader := &kubernetesfakes.FakeReader{}
			k8sClientReader.GetCalls(getCalls)

			cfg := telemetry.LabelCollectorConfig{
				K8sClientReader: k8sClientReader,
				Version:         "my-version",
				PodNSName: types.NamespacedName{
					Name:      "ngf-pod",
					Namespace: "nginx-gateway",
				},
			}

			c := telemetry.NewLabelCollector(cfg)

			labels, err := c.Collect(t.Context())
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.wantErrContain))
			g.Expect(labels).To(BeNil())
		})
	}
}
