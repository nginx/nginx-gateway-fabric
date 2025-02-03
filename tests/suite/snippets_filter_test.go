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
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

var _ = Describe("SnippetsFilter", Ordered, Label("functional", "snippets-filter"), func() {
	var (
		files = []string{
			"snippets-filter/cafe.yaml",
			"snippets-filter/gateway.yaml",
			"snippets-filter/grpc-backend.yaml",
		}

		namespace = "snippets-filter"
	)

	BeforeAll(func() {
		ns := &core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}

		Expect(resourceManager.Apply([]client.Object{ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
	})

	AfterAll(func() {
		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	When("SnippetsFilters are applied to the resources", func() {
		snippetsFilter := []string{
			"snippets-filter/valid-sf.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(snippetsFilter, namespace)).To(Succeed())
		})

		AfterAll(func() {
			Expect(resourceManager.DeleteFromFiles(snippetsFilter, namespace)).To(Succeed())
		})

		Specify("snippetsFilters are accepted", func() {
			snippetsFilterNames := []string{
				"all-contexts",
				"grpc-all-contexts",
			}

			for _, name := range snippetsFilterNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				err := waitForSnippetsFilterToBeAccepted(nsname)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("%s was not accepted", name))
			}
		})

		Context("verify working traffic", func() {
			It("should return a 200 response for HTTPRoute", func() {
				port := 80
				if portFwdPort != 0 {
					port = portFwdPort
				}
				baseURL := fmt.Sprintf("http://cafe.example.com:%d%s", port, "/coffee")

				Eventually(
					func() error {
						return expectRequestToSucceed(baseURL, address, "URI: /coffee")
					}).
					WithTimeout(timeoutConfig.RequestTimeout).
					WithPolling(500 * time.Millisecond).
					Should(Succeed())
			})
		})

		Context("nginx directives", func() {
			var conf *framework.Payload
			snippetsFilterFilePrefix := "/etc/nginx/includes/SnippetsFilter_"

			mainContext := fmt.Sprintf("%smain_", snippetsFilterFilePrefix)
			httpContext := fmt.Sprintf("%shttp_", snippetsFilterFilePrefix)
			httpServerContext := fmt.Sprintf("%shttp.server_", snippetsFilterFilePrefix)
			httpServerLocationContext := fmt.Sprintf("%shttp.server.location_", snippetsFilterFilePrefix)

			httpRouteSuffix := fmt.Sprintf("%s_all-contexts.conf", namespace)
			grpcRouteSuffix := fmt.Sprintf("%s_grpc-all-contexts.conf", namespace)

			BeforeAll(func() {
				podNames, err := framework.GetReadyNGFPodNames(k8sClient, ngfNamespace, releaseName, timeoutConfig.GetTimeout)
				Expect(err).ToNot(HaveOccurred())
				Expect(podNames).To(HaveLen(1))

				ngfPodName := podNames[0]

				conf, err = resourceManager.GetNginxConfig(ngfPodName, ngfNamespace)
				Expect(err).ToNot(HaveOccurred())
			})

			DescribeTable("are set properly for",
				func(expCfgs []framework.ExpectedNginxField) {
					for _, expCfg := range expCfgs {
						Expect(framework.ValidateNginxFieldExists(conf, expCfg)).To(Succeed())
					}
				},
				Entry("HTTPRoute", []framework.ExpectedNginxField{
					{
						Directive: "worker_priority",
						Value:     "0",
						File:      fmt.Sprintf("%s%s", mainContext, httpRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", mainContext, httpRouteSuffix),
						File:      "main.conf",
					},
					{
						Directive: "aio",
						Value:     "on",
						File:      fmt.Sprintf("%s%s", httpContext, httpRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpContext, httpRouteSuffix),
						File:      "http.conf",
					},
					{
						Directive: "auth_delay",
						Value:     "10s",
						File:      fmt.Sprintf("%s%s", httpServerContext, httpRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerContext, httpRouteSuffix),
						Server:    "cafe.example.com",
						File:      "http.conf",
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerLocationContext, httpRouteSuffix),
						File:      "http.conf",
						Location:  "/coffee",
						Server:    "cafe.example.com",
					},
					{
						Directive: "keepalive_time",
						Value:     "10s",
						File:      fmt.Sprintf("%s%s", httpServerLocationContext, httpRouteSuffix),
					},
				}),
				Entry("GRPCRoute", []framework.ExpectedNginxField{
					{
						Directive: "worker_shutdown_timeout",
						Value:     "120s",
						File:      fmt.Sprintf("%s%s", mainContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", mainContext, grpcRouteSuffix),
						File:      "main.conf",
					},
					{
						Directive: "types_hash_bucket_size",
						Value:     "64",
						File:      fmt.Sprintf("%s%s", httpContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpContext, grpcRouteSuffix),
						File:      "http.conf",
					},
					{
						Directive: "server_tokens",
						Value:     "on",
						File:      fmt.Sprintf("%s%s", httpServerContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerContext, grpcRouteSuffix),
						Server:    "*.example.com",
						File:      "http.conf",
					},
					{
						Directive: "tcp_nodelay",
						Value:     "on",
						File:      fmt.Sprintf("%s%s", httpServerLocationContext, grpcRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpServerLocationContext, grpcRouteSuffix),
						File:      "http.conf",
						Location:  "/helloworld.Greeter/SayHello",
						Server:    "*.example.com",
					},
				}),
			)
		})
	})

	When("SnippetsFilter is invalid", func() {
		Specify("if directives already present in the config are used", func() {
			files := []string{"snippets-filter/invalid-duplicate-sf.yaml"}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			nsname := types.NamespacedName{Name: "tea", Namespace: namespace}
			Expect(waitForHTTPRouteToHaveGatewayNotProgrammedCond(nsname)).To(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})

		Specify("if directives are provided in the wrong context", func() {
			files := []string{"snippets-filter/invalid-context-sf.yaml"}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			nsname := types.NamespacedName{Name: "soda", Namespace: namespace}
			Expect(waitForHTTPRouteToHaveGatewayNotProgrammedCond(nsname)).To(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})
	})
})

func waitForHTTPRouteToHaveGatewayNotProgrammedCond(httpRouteNsName types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout*2)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for HTTPRoute %q to have the condition Accepted/True/GatewayNotProgrammed\n",
		httpRouteNsName,
	)

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var hr v1.HTTPRoute
			var err error

			if err = k8sClient.Get(ctx, httpRouteNsName, &hr); err != nil {
				return false, err
			}

			if len(hr.Status.Parents) == 0 {
				return false, nil
			}

			if len(hr.Status.Parents) != 1 {
				return false, fmt.Errorf("httproute has %d parent statuses, expected 1", len(hr.Status.Parents))
			}

			parent := hr.Status.Parents[0]
			if parent.Conditions == nil {
				return false, fmt.Errorf("expected parent conditions to not be nil")
			}

			cond := parent.Conditions[1]
			if cond.Type != string(v1.GatewayConditionAccepted) {
				return false, fmt.Errorf("expected condition type to be Accepted, got %s", cond.Type)
			}

			if cond.Status != metav1.ConditionFalse {
				return false, fmt.Errorf("expected condition status to be False, got %s", cond.Status)
			}

			if cond.Reason != string(conditions.RouteReasonGatewayNotProgrammed) {
				return false, fmt.Errorf("expected condition reason to be GatewayNotProgrammed, got %s", cond.Reason)
			}
			return err == nil, err
		},
	)
}

func waitForSnippetsFilterToBeAccepted(snippetsFilterNsNames types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetStatusTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Waiting for SnippetsFilter %q to have the condition Accepted/True/Accepted\n",
		snippetsFilterNsNames,
	)

	return wait.PollUntilContextCancel(
		ctx,
		500*time.Millisecond,
		true, /* poll immediately */
		func(ctx context.Context) (bool, error) {
			var sf ngfAPI.SnippetsFilter
			var err error

			if err = k8sClient.Get(ctx, snippetsFilterNsNames, &sf); err != nil {
				return false, err
			}

			if len(sf.Status.Controllers) == 0 {
				return false, nil
			}

			if len(sf.Status.Controllers) != 1 {
				return false, fmt.Errorf("snippetsFilter has %d controller statuses, expected 1", len(sf.Status.Controllers))
			}

			status := sf.Status.Controllers[0]
			if status.ControllerName != ngfControllerName {
				return false, fmt.Errorf("expected controller name to be %s, got %s", ngfControllerName, status.ControllerName)
			}

			condition := status.Conditions[0]
			if condition.Type != string(ngfAPI.SnippetsFilterConditionTypeAccepted) {
				return false, fmt.Errorf("expected condition type to be Accepted, got %s", condition.Type)
			}

			if status.Conditions[0].Status != metav1.ConditionTrue {
				return false, fmt.Errorf("expected condition status to be %s, got %s", metav1.ConditionTrue, condition.Status)
			}

			if status.Conditions[0].Reason != string(ngfAPI.SnippetsFilterConditionReasonAccepted) {
				return false, fmt.Errorf(
					"expected condition reason to be %s, got %s",
					ngfAPI.SnippetsFilterConditionReasonAccepted,
					condition.Reason,
				)
			}

			return err == nil, err
		},
	)
}
