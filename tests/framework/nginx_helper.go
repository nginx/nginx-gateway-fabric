package framework

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MustGetNginxConfig fetches and parses the live NGINX configuration, failing the spec on error.
func MustGetNginxConfig(rm *ResourceManager, podName, ns string) *Payload {
	GinkgoHelper()
	conf, err := rm.GetNginxConfig(podName, ns, "")
	Expect(err).ToNot(HaveOccurred())
	return conf
}
