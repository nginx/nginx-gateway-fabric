package provisioner

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/apps/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	. "github.com/onsi/gomega"

	embeddedfiles "github.com/nginx/nginx-gateway-fabric"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/conditions"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/events"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/gatewayclass"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/internal/framework/status"
)

var _ = Describe("handler", func() {
	const (
		gcName = "test-gc"
	)
	var (
		handler *eventHandler

		statusUpdater *status.Updater
		k8sclient     client.Client
		crd           *metav1.PartialObjectMetadata
		gc            *gatewayv1.GatewayClass

		fakeTimeNow timeNowFunc
	)

	BeforeEach(OncePerOrdered, func() {
		scheme := runtime.NewScheme()

		Expect(gatewayv1.Install(scheme)).Should(Succeed())
		Expect(v1.AddToScheme(scheme)).Should(Succeed())
		Expect(apiext.AddToScheme(scheme)).Should(Succeed())

		k8sclient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(
				&gatewayv1.Gateway{},
				&gatewayv1.GatewayClass{},
			).
			Build()

		fakeTime := helpers.PrepareTimeForFakeClient(metav1.Now())
		fakeTimeNow = func() metav1.Time {
			return fakeTime
		}

		statusUpdater = status.NewUpdater(k8sclient, zap.New())

		// Add GatewayClass CRD to the cluster
		crd = &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "gatewayclasses.gateway.networking.k8s.io",
				Annotations: map[string]string{
					gatewayclass.BundleVersionAnnotation: gatewayclass.SupportedVersion,
				},
			},
		}

		err := k8sclient.Create(context.Background(), crd)
		Expect(err).ToNot(HaveOccurred())

		gc = &gatewayv1.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: gcName,
			},
		}
	})

	createGateway := func(gwNsName types.NamespacedName) *gatewayv1.Gateway {
		return &gatewayv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: gwNsName.Namespace,
				Name:      gwNsName.Name,
			},
			Spec: gatewayv1.GatewaySpec{
				GatewayClassName: gcName,
			},
		}
	}

	itShouldUpsertGatewayClass := func() {
		// Add GatewayClass to the cluster

		err := k8sclient.Create(context.Background(), gc)
		Expect(err).ToNot(HaveOccurred())

		// UpsertGatewayClass and CRD

		batch := []interface{}{
			&events.UpsertEvent{
				Resource: gc,
			},
			&events.UpsertEvent{
				Resource: crd,
			},
		}
		handler.HandleEventBatch(context.Background(), zap.New(), batch)

		// Ensure GatewayClass is accepted

		clusterGc := &gatewayv1.GatewayClass{}
		err = k8sclient.Get(context.Background(), client.ObjectKeyFromObject(gc), clusterGc)

		Expect(err).ToNot(HaveOccurred())

		expectedConditions := []metav1.Condition{
			{
				Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 0,
				LastTransitionTime: fakeTimeNow(),
				Reason:             "Accepted",
				Message:            "GatewayClass is accepted",
			},
			{
				Type:               string(gatewayv1.GatewayClassReasonSupportedVersion),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 0,
				LastTransitionTime: fakeTimeNow(),
				Reason:             "SupportedVersion",
				Message:            "Gateway API CRD versions are supported",
			},
		}

		Expect(clusterGc.Status.Conditions).To(Equal(expectedConditions))
	}

	itShouldUpsertGateway := func(gwNsName types.NamespacedName, seqNumber int64) {
		batch := []interface{}{
			&events.UpsertEvent{
				Resource: createGateway(gwNsName),
			},
		}

		handler.HandleEventBatch(context.Background(), zap.New(), batch)

		depNsName := types.NamespacedName{
			Namespace: "nginx-gateway",
			Name:      fmt.Sprintf("nginx-gateway-%d", seqNumber),
		}

		dep := &v1.Deployment{}
		err := k8sclient.Get(context.Background(), depNsName, dep)

		Expect(err).ToNot(HaveOccurred())

		Expect(dep.ObjectMeta.Namespace).To(Equal("nginx-gateway"))
		Expect(dep.ObjectMeta.Name).To(Equal(depNsName.Name))
		Expect(dep.Spec.Template.Spec.Containers[0].Args).To(ContainElement("static-mode"))
		expectedGwFlag := fmt.Sprintf("--gateway=%s", gwNsName.String())
		Expect(dep.Spec.Template.Spec.Containers[0].Args).To(ContainElement(expectedGwFlag))
		Expect(dep.Spec.Template.Spec.Containers[0].Args).To(ContainElement("--update-gatewayclass-status=false"))
		expectedLockFlag := fmt.Sprintf("--leader-election-lock-name=%s", gwNsName.Name)
		Expect(dep.Spec.Template.Spec.Containers[0].Args).To(ContainElement(expectedLockFlag))
	}

	itShouldUpsertCRD := func(version string, accepted bool) {
		updatedCRD := crd
		updatedCRD.Annotations[gatewayclass.BundleVersionAnnotation] = version

		err := k8sclient.Update(context.Background(), updatedCRD)
		Expect(err).ToNot(HaveOccurred())

		batch := []interface{}{
			&events.UpsertEvent{
				Resource: updatedCRD,
			},
		}

		handler.HandleEventBatch(context.Background(), zap.New(), batch)

		updatedGC := &gatewayv1.GatewayClass{}

		err = k8sclient.Get(context.Background(), client.ObjectKeyFromObject(gc), updatedGC)
		Expect(err).ToNot(HaveOccurred())

		var expConds []metav1.Condition
		if !accepted {
			expConds = []metav1.Condition{
				{
					Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 0,
					LastTransitionTime: fakeTimeNow(),
					Reason:             string(gatewayv1.GatewayClassReasonUnsupportedVersion),
					Message: fmt.Sprintf("Gateway API CRD versions are not supported. "+
						"Please install version %s", gatewayclass.SupportedVersion),
				},
				{
					Type:               string(gatewayv1.GatewayClassReasonSupportedVersion),
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 0,
					LastTransitionTime: fakeTimeNow(),
					Reason:             string(gatewayv1.GatewayClassReasonUnsupportedVersion),
					Message: fmt.Sprintf("Gateway API CRD versions are not supported. "+
						"Please install version %s", gatewayclass.SupportedVersion),
				},
			}
		} else {
			expConds = []metav1.Condition{
				{
					Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
					Status:             metav1.ConditionTrue,
					ObservedGeneration: 0,
					LastTransitionTime: fakeTimeNow(),
					Reason:             string(gatewayv1.GatewayClassReasonAccepted),
					Message:            "GatewayClass is accepted",
				},
				{
					Type:               string(gatewayv1.GatewayClassReasonSupportedVersion),
					Status:             metav1.ConditionFalse,
					ObservedGeneration: 0,
					LastTransitionTime: fakeTimeNow(),
					Reason:             string(gatewayv1.GatewayClassReasonUnsupportedVersion),
					Message: fmt.Sprintf("Gateway API CRD versions are not recommended. "+
						"Recommended version is %s", gatewayclass.SupportedVersion),
				},
			}
		}

		Expect(updatedGC.Status.Conditions).To(Equal(expConds))
	}

	itShouldPanicWhenUpsertingGateway := func(gwNsName types.NamespacedName) {
		batch := []interface{}{
			&events.UpsertEvent{
				Resource: createGateway(gwNsName),
			},
		}

		handle := func() {
			handler.HandleEventBatch(context.Background(), zap.New(), batch)
		}

		Expect(handle).Should(Panic())
	}

	Describe("Core cases", Ordered, func() {
		var gwNsName1, gwNsName2 types.NamespacedName

		BeforeAll(func() {
			gwNsName1 = types.NamespacedName{
				Namespace: "test-ns-1",
				Name:      "test-gw-1",
			}
			gwNsName2 = types.NamespacedName{
				Namespace: "test-ns-2",
				Name:      "test-gw-2",
			}

			handler = newEventHandler(
				gcName,
				statusUpdater,
				k8sclient,
				embeddedfiles.StaticModeDeploymentYAML,
				fakeTimeNow,
			)
		})

		When("upserting GatewayClass", func() {
			It("should make GatewayClass Accepted", func() {
				itShouldUpsertGatewayClass()
			})
		})

		When("upserting first Gateway", func() {
			It("should create first Deployment", func() {
				itShouldUpsertGateway(gwNsName1, 1)
			})
		})

		When("upserting first Gateway again", func() {
			It("must retain Deployment", func() {
				itShouldUpsertGateway(gwNsName1, 1)
			})
		})

		When("upserting second Gateway", func() {
			It("should create second Deployment", func() {
				itShouldUpsertGateway(gwNsName2, 2)
			})
		})

		When("deleting first Gateway", func() {
			It("should remove first Deployment", func() {
				batch := []interface{}{
					&events.DeleteEvent{
						Type:           &gatewayv1.Gateway{},
						NamespacedName: gwNsName1,
					},
				}

				handler.HandleEventBatch(context.Background(), zap.New(), batch)
				deps := &v1.DeploymentList{}

				err := k8sclient.List(context.Background(), deps)

				Expect(err).ToNot(HaveOccurred())
				Expect(deps.Items).To(HaveLen(1))
				Expect(deps.Items[0].ObjectMeta.Name).To(Equal("nginx-gateway-2"))
			})
		})

		When("deleting second Gateway", func() {
			It("should remove second Deployment", func() {
				batch := []interface{}{
					&events.DeleteEvent{
						Type:           &gatewayv1.Gateway{},
						NamespacedName: gwNsName2,
					},
				}

				handler.HandleEventBatch(context.Background(), zap.New(), batch)

				deps := &v1.DeploymentList{}

				err := k8sclient.List(context.Background(), deps)

				Expect(err).ToNot(HaveOccurred())
				Expect(deps.Items).To(BeEmpty())
			})
		})

		When("upserting Gateway for a different GatewayClass", func() {
			It("should not create Deployment", func() {
				gw := &gatewayv1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gw-3",
						Namespace: "test-ns-3",
					},
					Spec: gatewayv1.GatewaySpec{
						GatewayClassName: "some-class",
					},
				}

				batch := []interface{}{
					&events.UpsertEvent{
						Resource: gw,
					},
				}

				handler.HandleEventBatch(context.Background(), zap.New(), batch)

				deps := &v1.DeploymentList{}
				err := k8sclient.List(context.Background(), deps)

				Expect(err).ToNot(HaveOccurred())
				Expect(deps.Items).To(BeEmpty())
			})
		})

		When("upserting GatewayClass that is not set in command-line argument", func() {
			It("should set the proper status if this controller is referenced", func() {
				newGC := &gatewayv1.GatewayClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "unknown-gc",
					},
					Spec: gatewayv1.GatewayClassSpec{
						ControllerName: "test.example.com",
					},
				}

				err := k8sclient.Create(context.Background(), newGC)
				Expect(err).ToNot(HaveOccurred())

				batch := []interface{}{
					&events.UpsertEvent{
						Resource: newGC,
					},
					&events.UpsertEvent{
						Resource: crd,
					},
				}

				handler.HandleEventBatch(context.Background(), zap.New(), batch)

				unknownGC := &gatewayv1.GatewayClass{}
				err = k8sclient.Get(context.Background(), client.ObjectKeyFromObject(newGC), unknownGC)
				Expect(err).ToNot(HaveOccurred())

				expectedConditions := []metav1.Condition{
					{
						Type:               string(gatewayv1.GatewayClassReasonSupportedVersion),
						Status:             metav1.ConditionTrue,
						ObservedGeneration: 0,
						LastTransitionTime: fakeTimeNow(),
						Reason:             "SupportedVersion",
						Message:            "Gateway API CRD versions are supported",
					},
					{
						Type:               string(gatewayv1.GatewayClassConditionStatusAccepted),
						Status:             metav1.ConditionFalse,
						ObservedGeneration: 0,
						LastTransitionTime: fakeTimeNow(),
						Reason:             string(conditions.GatewayClassReasonGatewayClassConflict),
						Message:            conditions.GatewayClassMessageGatewayClassConflict,
					},
				}
				Expect(unknownGC.Status.Conditions).To(Equal(expectedConditions))
			})
		})

		When("upserting Gateway API CRD that is not a supported major version", func() {
			It("should set the SupportedVersion and Accepted statuses to false on GatewayClass", func() {
				itShouldUpsertCRD("v99.0.0", false /* accepted */)
			})
		})

		When("upserting Gateway API CRD that is not a supported minor version", func() {
			It("should set the SupportedVersion status to false and Accepted status to true on GatewayClass", func() {
				itShouldUpsertCRD("1.99.0", true /* accepted */)
			})
		})
	})

	Describe("Edge cases", func() {
		var gwNsName types.NamespacedName

		BeforeEach(func() {
			gwNsName = types.NamespacedName{
				Namespace: "test-ns",
				Name:      "test-gw",
			}

			handler = newEventHandler(
				gcName,
				statusUpdater,
				k8sclient,
				embeddedfiles.StaticModeDeploymentYAML,
				fakeTimeNow,
			)
		})

		DescribeTable("Edge cases for events",
			func(e interface{}) {
				batch := []interface{}{e}

				handle := func() {
					handler.HandleEventBatch(context.Background(), zap.New(), batch)
				}

				Expect(handle).Should(Panic())
			},
			Entry("should panic for an unknown event type",
				&struct{}{}),
			Entry("should panic for an unknown type of resource in upsert event",
				&events.UpsertEvent{
					Resource: &gatewayv1.HTTPRoute{},
				}),
			Entry("should panic for an unknown type of resource in delete event",
				&events.DeleteEvent{
					Type: &gatewayv1.HTTPRoute{},
				}),
		)

		When("upserting Gateway when GatewayClass doesn't exist", func() {
			It("should panic", func() {
				itShouldPanicWhenUpsertingGateway(gwNsName)
			})
		})

		When("upserting Gateway when Deployment can't be created", func() {
			It("should panic", func() {
				itShouldUpsertGatewayClass()

				// Create a deployment so that the Handler will fail to create it because it already exists.

				dep := &v1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "nginx-gateway",
						Name:      "nginx-gateway-1",
					},
				}

				err := k8sclient.Create(context.Background(), dep)
				Expect(err).ToNot(HaveOccurred())

				itShouldPanicWhenUpsertingGateway(gwNsName)
			})
		})

		When("deleting Gateway when Deployment can't be deleted", func() {
			It("should panic", func() {
				itShouldUpsertGatewayClass()
				itShouldUpsertGateway(gwNsName, 1)

				// Delete the deployment so that the Handler will fail to delete it because it doesn't exist.

				dep := &v1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "nginx-gateway",
						Name:      "nginx-gateway-1",
					},
				}

				err := k8sclient.Delete(context.Background(), dep)
				Expect(err).ToNot(HaveOccurred())

				batch := []interface{}{
					&events.DeleteEvent{
						Type:           &gatewayv1.Gateway{},
						NamespacedName: gwNsName,
					},
				}

				handle := func() {
					handler.HandleEventBatch(context.Background(), zap.New(), batch)
				}

				Expect(handle).Should(Panic())
			})
		})

		When("deleting GatewayClass", func() {
			It("should panic", func() {
				itShouldUpsertGatewayClass()

				batch := []interface{}{
					&events.DeleteEvent{
						Type: &gatewayv1.GatewayClass{},
						NamespacedName: types.NamespacedName{
							Name: gcName,
						},
					},
				}

				handle := func() {
					handler.HandleEventBatch(context.Background(), zap.New(), batch)
				}

				Expect(handle).Should(Panic())
			})
		})

		When("upserting Gateway with broken static Deployment YAML", func() {
			It("it should panic", func() {
				handler = newEventHandler(
					gcName,
					statusUpdater,
					k8sclient,
					[]byte("broken YAML"),
					fakeTimeNow,
				)

				itShouldUpsertGatewayClass()
				itShouldPanicWhenUpsertingGateway(types.NamespacedName{Namespace: "test-ns", Name: "test-gw"})
			})
		})
	})
})
