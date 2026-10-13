package framework

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// GetNginxConf fetches and parses the live NGINX configuration from the given pod.
func GetNginxConf(rm *ResourceManager, podName, ns string) *Payload {
	GinkgoHelper()
	conf, err := rm.GetNginxConfig(podName, ns, "")
	Expect(err).ToNot(HaveOccurred())
	return conf
}
