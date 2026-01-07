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
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"

	ngfAPI "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	"github.com/nginx/nginx-gateway-fabric/v2/tests/framework"
)

var _ = Describe("SnippetsFilter", Ordered, Label("functional", "snippets-filter"), func() {
	var (
		files = []string{
			"snippets-filter/cafe.yaml",
			"snippets-filter/gateway.yaml",
			"snippets-filter/grpc-backend.yaml",
		}

		namespace = "snippets-filter"

		nginxPodName  string
		gatewayNsName = types.NamespacedName{Name: "gateway", Namespace: namespace}
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

		nginxPodNames, err := resourceManager.GetReadyNginxPodNames(
			namespace,
			timeoutConfig.GetStatusTimeout,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(nginxPodNames).To(HaveLen(1))

		nginxPodName = nginxPodNames[0]

		setUpPortForward(nginxPodName, namespace)
	})

	AfterAll(func() {
		cleanUpPortForward()

		Expect(resourceManager.DeleteNamespace(namespace)).To(Succeed())
	})

	When("SnippetsFilters are applied to the resources", func() {
		snippetsFilter := []string{
			"snippets-filter/valid-sf.yaml",
		}

		BeforeAll(func() {
			Expect(resourceManager.ApplyFromFiles(snippetsFilter, namespace)).To(Succeed())
			Expect(resourceManager.WaitForAppsToBeReady(namespace)).To(Succeed())
		})

		AfterAll(func() {
			framework.AddNginxLogsAndEventsToReport(resourceManager, namespace)
			Expect(resourceManager.DeleteFromFiles(snippetsFilter, namespace)).To(Succeed())
		})

		Specify("snippetsFilters are accepted", func() {
			snippetsFilterNames := []string{
				"all-contexts",
				"grpc-all-contexts",
			}

			for _, name := range snippetsFilterNames {
				nsname := types.NamespacedName{Name: name, Namespace: namespace}

				Eventually(checkForSnippetsFilterToBeAccepted).
					WithArguments(nsname).
					WithTimeout(timeoutConfig.GetStatusTimeout).
					WithPolling(500*time.Millisecond).
					Should(Succeed(), fmt.Sprintf("%s was not accepted", name))
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
						return framework.ExpectRequestToSucceed(timeoutConfig.RequestTimeout, baseURL, address, "URI: /coffee")
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
				var err error
				conf, err = resourceManager.GetNginxConfig(nginxPodName, namespace, "")
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
						Value:     "off",
						File:      fmt.Sprintf("%s%s", httpContext, httpRouteSuffix),
					},
					{
						Directive: "include",
						Value:     fmt.Sprintf("%s%s", httpContext, httpRouteSuffix),
						File:      "http.conf",
					},
					{
						Directive: "auth_delay",
						Value:     "0s",
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
						Value:     "1h",
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

			Eventually(checkGatewayToHaveGatewayNotProgrammedCond).
				WithArguments(gatewayNsName).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})

		Specify("if directives are provided in the wrong context", func() {
			files := []string{"snippets-filter/invalid-context-sf.yaml"}

			Expect(resourceManager.ApplyFromFiles(files, namespace)).To(Succeed())

			Eventually(checkGatewayToHaveGatewayNotProgrammedCond).
				WithArguments(gatewayNsName).
				WithTimeout(timeoutConfig.GetStatusTimeout).
				WithPolling(500 * time.Millisecond).
				Should(Succeed())

			Expect(resourceManager.DeleteFromFiles(files, namespace)).To(Succeed())
		})
	})
})

func checkGatewayToHaveGatewayNotProgrammedCond(gatewayNsName types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Checking for Gateway %q to have the condition Programmed/False/Invalid \n",
		gatewayNsName,
	)

	var gw v1.Gateway
	var err error

	if err = resourceManager.Get(ctx, gatewayNsName, &gw); err != nil {
		return err
	}

	gwStatus := gw.Status
	if gwStatus.Conditions == nil {
		nilConditionErr := fmt.Errorf("expected gateway conditions to not be nil")
		GinkgoWriter.Printf("ERROR: %v\n", nilConditionErr)

		return nilConditionErr
	}

	for i := range gwStatus.Conditions {
		GinkgoWriter.Printf("Gateway condition %d: Type=%s, Status=%s, Reason=%s\n",
			i, gwStatus.Conditions[i].Type, gwStatus.Conditions[i].Status, gwStatus.Conditions[i].Reason)
	}

	cond := gwStatus.Conditions[1]
	if cond.Type != string(v1.GatewayConditionProgrammed) {
		wrongTypeErr := fmt.Errorf("expected condition type to be Programmed, got %s", cond.Type)
		GinkgoWriter.Printf("ERROR: %v\n", wrongTypeErr)

		return wrongTypeErr
	}

	if cond.Status != metav1.ConditionFalse {
		wrongStatusErr := fmt.Errorf("expected condition status to be False, got %s", cond.Status)
		GinkgoWriter.Printf("ERROR: %v\n", wrongStatusErr)

		return wrongStatusErr
	}

	if cond.Reason != string(v1.GatewayReasonInvalid) {
		wrongReasonErr := fmt.Errorf("expected condition reason to be GatewayReasonInvalid, got %s", cond.Reason)
		GinkgoWriter.Printf("ERROR: %v\n", wrongReasonErr)

		return wrongReasonErr
	}

	return nil
}

func checkForSnippetsFilterToBeAccepted(snippetsFilterNsNames types.NamespacedName) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutConfig.GetTimeout)
	defer cancel()

	GinkgoWriter.Printf(
		"Checking for SnippetsFilter %q to have the condition Accepted/True/Accepted\n",
		snippetsFilterNsNames,
	)

	var sf ngfAPI.SnippetsFilter
	var err error

	if err = resourceManager.Get(ctx, snippetsFilterNsNames, &sf); err != nil {
		return err
	}

	return framework.CheckFilterAccepted(
		sf,
		ngfControllerName,
		framework.SnippetsFilterControllers,
		(string)(ngfAPI.SnippetsFilterConditionTypeAccepted),
		(string)(ngfAPI.SnippetsFilterConditionReasonAccepted),
	)
}
