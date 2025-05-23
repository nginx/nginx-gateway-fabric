package status

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

// We only use one resource type in this test - GatewayClass.
// It is enough, as the Updater is resource-agnostic.
// GatewayClass is used because it has a simple status.
var _ = Describe("LeaderAwareGroupUpdater", func() {
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
		const group1 = "group1"
		const group2 = "group2"

		var (
			updater *LeaderAwareGroupUpdater

			group1GCNames = []string{"one-first", "one-second"}
			group2GCNames = []string{"two-first", "two-second"}

			allGCNames = append(append([]string{}, group1GCNames...), group2GCNames...)
		)

		BeforeAll(func() {
			updater = NewLeaderAwareGroupUpdater(NewUpdater(k8sClient, logr.Discard()))

			for _, name := range allGCNames {
				gc := createGC(name)
				Expect(k8sClient.Create(context.Background(), gc)).To(Succeed())
			}
		})

		prepareReq := func(name string, condType string, updateNeeded bool) UpdateRequest {
			setter := func(_ client.Object) bool { return false }

			if updateNeeded {
				setter = func(obj client.Object) bool {
					gc, ok := obj.(*v1.GatewayClass)
					Expect(ok).To(BeTrue(), "obj is not a *v1.GatewayClass")
					gc.Status = createGCStatus(condType)
					return true
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

		prepareReqs := func(names []string, condType string) []UpdateRequest {
			reqs := make([]UpdateRequest, 0, len(names))
			for _, name := range names {
				reqs = append(reqs, prepareReq(name, condType, updateNeeded))
			}
			return reqs
		}

		testStatuses := func(names []string, condType string) {
			for _, name := range names {
				var gc v1.GatewayClass
				Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, &gc)).To(Succeed())
				Expect(gc.Status).To(Equal(createGCStatus(condType)))
			}
		}

		testNoStatuses := func(names []string) {
			for _, name := range names {
				var gc v1.GatewayClass
				Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, &gc)).To(Succeed())
				Expect(gc.Status).To(Equal(v1.GatewayClassStatus{}))
			}
		}

		When("updater is disabled", func() {
			It("should save requests for later", func() {
				reqs1 := prepareReqs(group1GCNames, "TestAllSaveForLater")
				updater.UpdateGroup(context.Background(), group1, reqs1...)

				reqs2 := prepareReqs(group2GCNames, "TestAllSaveForLater")
				updater.UpdateGroup(context.Background(), group2, reqs2...)

				testNoStatuses(allGCNames)
			})

			When("passing no update requests", func() {
				It("should clear saved requests of group2", func() {
					updater.UpdateGroup(context.Background(), group2)
				})
			})
		})

		When("updater is enabled", func() {
			It("should update statuses from saved requests", func() {
				updater.Enable(context.Background())

				testStatuses(group1GCNames, "TestAllSaveForLater")
				testNoStatuses(group2GCNames)
			})

			When("passing no update requests", func() {
				It("should not update statuses of group1", func() {
					updater.UpdateGroup(context.Background(), group1)
					testStatuses(group1GCNames, "TestAllSaveForLater")
				})
			})

			It("should update statuses of all groups", func() {
				reqs1 := prepareReqs(group1GCNames, "TestAll")
				updater.UpdateGroup(context.Background(), group1, reqs1...)

				reqs2 := prepareReqs(group2GCNames, "TestAll")
				updater.UpdateGroup(context.Background(), group2, reqs2...)

				testStatuses(allGCNames, "TestAll")
			})
		})

		When("updater is enabled second time", func() {
			It("should panic", func() {
				Expect(func() {
					updater.Enable(context.Background())
				}).Should(Panic())
			})
		})

		// cases of canceling the context and updates that are not needed
		// are covered in the regular Updater tests
	})
})
