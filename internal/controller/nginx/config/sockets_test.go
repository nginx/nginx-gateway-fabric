package config

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetSocketNameTLS(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	g.Expect(getSocketNameTLS(800, "*.cafe.example.com")).To(Equal("unix:/var/run/nginx/*.cafe.example.com-800.sock"))
	g.Expect(getSocketNameTLS(8443, "")).To(Equal("unix:/var/run/nginx/8443.sock"))
}

func TestGetSocketNameTLSTerminate(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	g.Expect(getSocketNameTLSTerminate(800, "*.cafe.example.com")).To(
		Equal("unix:/var/run/nginx/*.cafe.example.com-800-terminate.sock"),
	)
	g.Expect(getSocketNameTLSTerminate(8443, "")).To(Equal("unix:/var/run/nginx/8443-terminate.sock"))
}

func TestGetSocketNameHTTPS(t *testing.T) {
	t.Parallel()
	res := getSocketNameHTTPS(800)

	g := NewGomegaWithT(t)
	g.Expect(res).To(Equal("unix:/var/run/nginx/https800.sock"))
}

func TestGetTLSPassthroughVarName(t *testing.T) {
	t.Parallel()
	res := getTLSPassthroughVarName(800)

	g := NewGomegaWithT(t)
	g.Expect(res).To(Equal("$dest800"))
}
