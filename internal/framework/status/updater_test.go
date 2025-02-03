package status

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/kinds"
)

func createGC(name string) *v1.GatewayClass {
	return &v1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       kinds.GatewayClass,
			APIVersion: "gateway.networking.k8s.io/v1",
		},
	}
}

func createGCStatus(condType string) v1.GatewayClassStatus {
	return v1.GatewayClassStatus{
		Conditions: createConditions(condType),
	}
}

var currTime = helpers.PrepareTimeForFakeClient(metav1.Now())

func createConditions(
	condType string,
) []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               condType,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: 1,
			LastTransitionTime: currTime,
			Reason:             "TestReason1",
			Message:            "Test message1",
		},
		{
			Type:               condType,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: 1,
			LastTransitionTime: currTime,
			Reason:             "TestReason2",
			Message:            "Test message2",
		},
	}
}

const (
	updateNeeded    = true
	updateNotNeeded = false
)

func prepareReq(name string, condType string, updateNeeded bool) UpdateRequest {
	var setter Setter
	if updateNeeded {
		setter = func(obj client.Object) bool {
			gc, ok := obj.(*v1.GatewayClass)
			Expect(ok).To(BeTrue(), "obj is not a *v1.GatewayClass")
			gc.Status = createGCStatus(condType)
			return true
		}
	} else {
		setter = func(_ client.Object) bool {
			return false
		}
	}

	return UpdateRequest{
		NsName: types.NamespacedName{
			Name: name,
		},
		ResourceType: &v1.GatewayClass{},
		Setter:       setter,
	}
}

// We only use one resource type in this test - GatewayClass.
// It is enough, as the Updater is resource-agnostic.
// GatewayClass is used because it has a simple status.
var _ = Describe("Updater", func() {
	var k8sClient client.Client

	BeforeEach(OncePerOrdered, func() {
		scheme := runtime.NewScheme()

		Expect(v1.Install(scheme)).Should(Succeed())

		k8sClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(
				&v1.GatewayClass{},
			).
			Build()
	})

	Describe("Process status updates", Ordered, func() {
		var (
			updater *Updater

			gcNames = []string{"first", "second"}
		)

		BeforeAll(func() {
			updater = NewUpdater(k8sClient, zap.New())

			for _, name := range gcNames {
				gc := createGC(name)
				Expect(k8sClient.Create(context.Background(), gc)).Should(Succeed())
			}
		})

		testStatus := func(name string, condType string) {
			var gc v1.GatewayClass

			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, &gc)
			Expect(err).ToNot(HaveOccurred())
			Expect(gc.Status).To(Equal(createGCStatus(condType)))
		}

		It("should update the status of GatewayClasses individually", func() {
			for _, name := range gcNames {
				req := prepareReq(name, "TestIndividually", updateNeeded)
				updater.Update(context.Background(), req)
				testStatus(name, "TestIndividually")
			}
		})

		It("should update the status of all GatewayClasses", func() {
			reqs := make([]UpdateRequest, 0, len(gcNames))

			for _, name := range gcNames {
				reqs = append(reqs, prepareReq(name, "TestAll", updateNeeded))
			}

			updater.Update(context.Background(), reqs...)

			for _, name := range gcNames {
				testStatus(name, "TestAll")
			}
		})

		When("there are no updates", func() {
			It("should not update the status of GatewayClasses", func() {
				updater.Update(context.Background())

				for _, name := range gcNames {
					// condType from the last successful update should be present
					testStatus(name, "TestAll")
				}
			})
		})

		When("the context is canceled", func() {
			It("should not update the status of GatewayClasses", func() {
				reqs := make([]UpdateRequest, 0, len(gcNames))

				for _, name := range gcNames {
					reqs = append(reqs, prepareReq(name, "TestContextDone", updateNeeded))
				}

				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				updater.Update(ctx, reqs...)

				for _, name := range gcNames {
					// condType from the last successful update should be present
					testStatus(name, "TestAll")
				}
			})
		})

		When("the update is not needed", func() {
			It("should not update the status of GatewayClasses", func() {
				reqs := make([]UpdateRequest, 0, len(gcNames))

				for _, name := range gcNames {
					reqs = append(reqs, prepareReq(name, "TestNotNeeded", updateNotNeeded))
				}

				updater.Update(context.Background(), reqs...)

				for _, name := range gcNames {
					// condType from the last successful update should be present
					testStatus(name, "TestAll")
				}
			})
		})
	})
})
