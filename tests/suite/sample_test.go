package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nginx/nginx-gateway-fabric/tests/framework"
)

var _ = Describe("Basic test example", Label("functional"), func() {
	files := []string{
		"hello-world/apps.yaml",
		"hello-world/gateway.yaml",
		"hello-world/routes.yaml",
	}

	var ns core.Namespace

	BeforeEach(func() {
		ns = core.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "helloworld",
			},
		}

		Expect(resourceManager.Apply([]client.Object{&ns})).To(Succeed())
		Expect(resourceManager.ApplyFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.WaitForAppsToBeReady(ns.Name)).To(Succeed())
	})

	AfterEach(func() {
		Expect(resourceManager.DeleteFromFiles(files, ns.Name)).To(Succeed())
		Expect(resourceManager.DeleteNamespace(ns.Name)).To(Succeed())
	})

	It("sends traffic", func() {
		url := "http://foo.example.com/hello"
		if portFwdPort != 0 {
			url = fmt.Sprintf("http://foo.example.com:%s/hello", strconv.Itoa(portFwdPort))
		}

		Eventually(
			func() error {
				status, body, err := framework.Get(url, address, timeoutConfig.RequestTimeout)
				if err != nil {
					return err
				}
				if status != http.StatusOK {
					return fmt.Errorf("status not 200; got %d", status)
				}
				expBody := "URI: /hello"
				if !strings.Contains(body, expBody) {
					return fmt.Errorf("bad body: got %s; expected %s", body, expBody)
				}
				return nil
			}).
			WithTimeout(timeoutConfig.RequestTimeout).
			WithPolling(500 * time.Millisecond).
			Should(Succeed())
	})
})
