package controller_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/controller/controllerfakes"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
)

type getFunc func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error

type result struct {
	err             error
	reconcileResult reconcile.Result
}

var _ = Describe("Reconciler", func() {
	var (
		rec        *controller.Reconciler
		fakeGetter *controllerfakes.FakeGetter
		eventCh    chan interface{}

		hr1NsName = types.NamespacedName{
			Namespace: "test",
			Name:      "hr-1",
		}

		hr1 = &v1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: hr1NsName.Namespace,
				Name:      hr1NsName.Name,
			},
		}

		hr2NsName = types.NamespacedName{
			Namespace: "test",
			Name:      "hr-2",
		}

		hr2 = &v1.HTTPRoute{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: hr2NsName.Namespace,
				Name:      hr2NsName.Name,
			},
		}
	)

	getReturnsHRForHR := func(hr *v1.HTTPRoute) getFunc {
		return func(
			_ context.Context,
			nsname types.NamespacedName,
			object client.Object,
			_ ...client.GetOption,
		) error {
			Expect(object).To(BeAssignableToTypeOf(&v1.HTTPRoute{}))
			Expect(nsname).To(Equal(client.ObjectKeyFromObject(hr)))
			Expect(hr).ToNot(BeNil())

			hrObj, ok := object.(*v1.HTTPRoute)
			Expect(ok).To(BeTrue(), "object is not *v1.HTTPRoute")
			hr.DeepCopyInto(hrObj)

			return nil
		}
	}

	getReturnsNotFoundErrorForHR := func(hr *v1.HTTPRoute) getFunc {
		return func(
			_ context.Context,
			nsname types.NamespacedName,
			object client.Object,
			_ ...client.GetOption,
		) error {
			Expect(object).To(BeAssignableToTypeOf(&v1.HTTPRoute{}))
			Expect(nsname).To(Equal(client.ObjectKeyFromObject(hr)))

			return apierrors.NewNotFound(schema.GroupResource{}, "not found")
		}
	}

	startReconcilingWithContext := func(ctx context.Context, nsname types.NamespacedName) <-chan result {
		resultCh := make(chan result)

		go func() {
			defer GinkgoRecover()

			res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: nsname})
			resultCh <- result{err: err, reconcileResult: res}

			close(resultCh)
		}()

		return resultCh
	}

	startReconciling := func(nsname types.NamespacedName) <-chan result {
		return startReconcilingWithContext(context.Background(), nsname)
	}

	BeforeEach(func() {
		fakeGetter = &controllerfakes.FakeGetter{}
		eventCh = make(chan interface{})
	})

	Describe("Normal cases", func() {
		testUpsert := func(hr *v1.HTTPRoute) {
			fakeGetter.GetCalls(getReturnsHRForHR(hr))

			resultCh := startReconciling(client.ObjectKeyFromObject(hr))

			Eventually(eventCh).Should(Receive(Equal(&events.UpsertEvent{Resource: hr})))
			Eventually(resultCh).Should(Receive(Equal(result{err: nil, reconcileResult: reconcile.Result{}})))
		}

		testDelete := func(hr *v1.HTTPRoute) {
			fakeGetter.GetCalls(getReturnsNotFoundErrorForHR(hr))

			resultCh := startReconciling(client.ObjectKeyFromObject(hr))

			Eventually(eventCh).Should(Receive(Equal(&events.DeleteEvent{
				NamespacedName: client.ObjectKeyFromObject(hr),
				Type:           &v1.HTTPRoute{},
			})))
			Eventually(resultCh).Should(Receive(Equal(result{err: nil, reconcileResult: reconcile.Result{}})))
		}

		When("Reconciler doesn't have a filter", func() {
			BeforeEach(func() {
				rec = controller.NewReconciler(controller.ReconcilerConfig{
					Getter:     fakeGetter,
					ObjectType: &v1.HTTPRoute{},
					EventCh:    eventCh,
				})
			})

			It("should upsert HTTPRoute", func() {
				testUpsert(hr1)
			})

			It("should delete HTTPRoute", func() {
				testDelete(hr1)
			})
		})

		When("Reconciler has a NamespacedNameFilter", func() {
			BeforeEach(func() {
				filter := func(nsname types.NamespacedName) (bool, string) {
					if nsname != hr1NsName {
						return false, "ignore"
					}
					return true, ""
				}

				rec = controller.NewReconciler(controller.ReconcilerConfig{
					Getter:               fakeGetter,
					ObjectType:           &v1.HTTPRoute{},
					EventCh:              eventCh,
					NamespacedNameFilter: filter,
				})
			})

			When("HTTPRoute is not ignored", func() {
				It("should upsert HTTPRoute", func() {
					testUpsert(hr1)
				})

				It("should delete HTTPRoute", func() {
					testDelete(hr1)
				})
			})

			When("HTTPRoute is ignored", func() {
				It("should not upsert HTTPRoute", func() {
					fakeGetter.GetCalls(getReturnsHRForHR(hr2))

					resultCh := startReconciling(hr2NsName)

					Consistently(eventCh).ShouldNot(Receive())
					Eventually(resultCh).Should(Receive(Equal(result{err: nil, reconcileResult: reconcile.Result{}})))
				})

				It("should not delete HTTPRoute", func() {
					fakeGetter.GetCalls(getReturnsNotFoundErrorForHR(hr2))

					resultCh := startReconciling(hr2NsName)

					Consistently(eventCh).ShouldNot(Receive())
					Eventually(resultCh).Should(Receive(Equal(result{err: nil, reconcileResult: reconcile.Result{}})))
				})
			})
		})
	})

	Describe("Edge cases", func() {
		BeforeEach(func() {
			rec = controller.NewReconciler(controller.ReconcilerConfig{
				Getter:     fakeGetter,
				ObjectType: &v1.HTTPRoute{},
				EventCh:    eventCh,
			})
		})

		It("should not reconcile when Getter returns error", func() {
			getError := errors.New("get error")
			fakeGetter.GetReturns(getError)

			resultCh := startReconciling(hr1NsName)

			Consistently(eventCh).ShouldNot(Receive())
			Eventually(resultCh).Should(Receive(Equal(result{err: getError, reconcileResult: reconcile.Result{}})))
		})

		DescribeTable("Reconciler should not block when ctx is done",
			func(get getFunc, nsname types.NamespacedName) {
				fakeGetter.GetCalls(get)

				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				resultCh := startReconcilingWithContext(ctx, nsname)

				Consistently(eventCh).ShouldNot(Receive())
				Expect(resultCh).To(Receive(Equal(result{err: nil, reconcileResult: reconcile.Result{}})))
			},
			Entry("Upserting HTTPRoute", getReturnsHRForHR(hr1), hr1NsName),
			Entry("Deleting HTTPRoute", getReturnsNotFoundErrorForHR(hr1), hr1NsName),
		)
	})
})
