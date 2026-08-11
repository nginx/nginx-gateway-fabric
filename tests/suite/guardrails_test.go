package main

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

// The mock (tests/guardrails-mock) plays two roles behind two Services in the test namespace:
//   - guardrails-api: POST /backend/v1/scans, flagging any input containing the sentinel "BLOCKME".
//   - mock-llm:       POST /v1/completions, echoing the prompt back in OpenAI completion format so
//     the response path can inspect model output carrying the sentinel.
var _ = Describe("Guardrails (PayloadProcessor)", Ordered, Label("functional", "guardrails"), func() {
	var (
		appFiles = []string{
			"guardrails/apps.yaml",
			"guardrails/gateway.yaml",
			"guardrails/routes.yaml",
		}
		routePolicyFile = []string{
			"guardrails/payload-processor.yaml",
		}
		gatewayPolicyFile = []string{
			"guardrails/payload-processor-gateway.yaml",
		}

		namespace     = "guardrails"
		nginxPodName  string
		completionURL string
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(appFiles, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
			namespace,
			timeoutConfig.GetStatusTimeout,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))

		nginxPodName = nginxPodNames[0]
		setUpPortForward(nginxPodName, namespace)

		port := helpers.BuildPortFwdPort(80, portFwdPort)
		completionURL = helpers.BuildPortFwdURL("http://llm.example.com:%d/v1/completions", port)
	})

	AfterAll(func() {
		framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
		cleanUpPortForward()

		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	runGuardrailsAssertions := func() {
		Context("nginx directives", func() {
			It("enables the guardrails filter on the route location", func() {
				Eventually(func() error {
					conf, err := resourceManager.GetNginxConfig(nginxPodName, namespace, "")
					if err != nil {
						return err
					}

					return framework.ValidateNginxFieldExists(conf, framework.ExpectedNginxField{
						Directive: "guardrails_filter",
						Value:     "on",
						File:      "http.conf",
						Server:    "llm.example.com",
						Location:  "/v1/completions",
					})
				}).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("traffic", func() {
			It("allows a clean prompt and returns the completion", func() {
				payload := `{"prompt":"tell me a joke"}`
				Eventually(
					func() error {
						return framework.ExpectPostToSucceed(
							timeoutConfig.RequestTimeout,
							completionURL,
							address,
							payload,
							"echo: tell me a joke",
						)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})

			It("blocks a prompt flagged on the request path with 403", func() {
				// The sentinel in the prompt causes the request-path scan to flag the input before it
				// ever reaches the LLM. The module returns 403 with an invalid_request_error body.
				// "BLOCKME" must match BLOCK_SENTINEL set in manifests/guardrails/apps.yaml.
				payload := `{"prompt":"please BLOCKME now"}`
				Eventually(
					func() error {
						return framework.Expect403Response(
							timeoutConfig.RequestTimeout,
							completionURL,
							address,
							payload,
							"invalid_request_error",
						)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())

				// The module composes the backend-supplied scannerResults[].message into error.message
				// (see the ai-guardrails module README). Assert the flagged message reaches the client
				// to cover that composition path. "blocked by test guardrail" must match BLOCK_MESSAGE's
				// default in tests/guardrails-mock/main.go
				Expect(framework.Expect403Response(
					timeoutConfig.RequestTimeout,
					completionURL,
					address,
					payload,
					"blocked by test guardrail",
				)).To(Succeed())
			})

			It("blocks a response flagged on the response path with 403", func() {
				// The response sentinel ("BLOCKRESP") passes the request-path scan untouched, reaches
				// the mock LLM, and is echoed into the completion. The response-path scan then flags the
				// model output, and the module blocks with a 403 carrying an api_error body (distinct
				// from the request-path invalid_request_error).
				// "BLOCKRESP" must match RESPONSE_BLOCK_SENTINEL set in manifests/guardrails/apps.yaml.
				payload := `{"prompt":"produce BLOCKRESP in the answer"}`
				Eventually(
					func() error {
						return framework.Expect403Response(
							timeoutConfig.RequestTimeout,
							completionURL,
							address,
							payload,
							"api_error",
						)
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})
	}

	Context("attached to an HTTPRoute", Ordered, func() {
		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(routePolicyFile, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(routePolicyFile, namespace)).To(Succeed())
		})

		Specify("the PayloadProcessor is accepted with an HTTPRoute ancestor", func() {
			ppNsName := types.NamespacedName{Name: "llm-guardrails", Namespace: namespace}

			err := waitForPayloadProcessorStatus(
				ppNsName,
				gatewayv1.LocalPolicyTargetReference{
					Group: gatewayv1.GroupName,
					Kind:  gatewayv1.Kind("HTTPRoute"),
					Name:  "llm-route",
				},
				metav1.ConditionTrue,
				gatewayv1.PolicyReasonAccepted,
			)
			Expect(err).ToNot(HaveOccurred(), "llm-guardrails was not accepted")
		})

		runGuardrailsAssertions()
	})

	Context("attached to a Gateway", Ordered, func() {
		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(gatewayPolicyFile, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(gatewayPolicyFile, namespace)).To(Succeed())
		})

		Specify("the PayloadProcessor is accepted with a Gateway ancestor", func() {
			ppNsName := types.NamespacedName{Name: "gw-guardrails", Namespace: namespace}

			err := waitForPayloadProcessorStatus(
				ppNsName,
				gatewayv1.LocalPolicyTargetReference{
					Group: gatewayv1.GroupName,
					Kind:  gatewayv1.Kind("Gateway"),
					Name:  "gateway",
				},
				metav1.ConditionTrue,
				gatewayv1.PolicyReasonAccepted,
			)
			Expect(err).ToNot(HaveOccurred(), "gw-guardrails was not accepted")
		})

		runGuardrailsAssertions()
	})
})

func waitForPayloadProcessorStatus(
	ppNsName types.NamespacedName,
	targetRef gatewayv1.LocalPolicyTargetReference,
	condStatus metav1.ConditionStatus,
	condReason gatewayv1.PolicyConditionReason,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout*2)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for PayloadProcessor %q to have ancestor %s/%s with condition %q/%q\n",
		ppNsName,
		targetRef.Kind,
		targetRef.Name,
		condStatus,
		condReason,
	)

	return wait.PollUntilContextCancel(
		ctx,
		2000*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var pp ngfAPI.PayloadProcessor
			var err error

			if err := resourceManager.Get(ctx, ppNsName, &pp); err != nil {
				return false, err
			}

			if len(pp.Status.Ancestors) == 0 {
				GinkgoWriter.Printf("PayloadProcessor %q does not have an ancestor status yet\n", pp)

				return false, nil
			}

			if len(pp.Status.Ancestors) != 1 {
				tooManyAncestorsErr := fmt.Errorf(
					"policy has %d ancestors, expected 1",
					len(pp.Status.Ancestors),
				)
				GinkgoWriter.Printf("ERROR: %v\n", tooManyAncestorsErr)

				return false, tooManyAncestorsErr
			}

			ancestor := pp.Status.Ancestors[0]

			if err := ancestorMustEqualTargetRef(ancestor, targetRef, ppNsName.Namespace); err != nil {
				return false, err
			}

			err = ancestorStatusMustHaveAcceptedCondition(ancestor, condStatus, condReason)
			return err == nil, err
		},
	)
}
