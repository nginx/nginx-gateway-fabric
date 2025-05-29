package config

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetSocketNameTLS(t *testing.T) {
	t.Parallel()
	res := getSocketNameTLS(800, "*.cafe.example.com")

	g := NewGomegaWithT(t)
	g.Expect(res).To(Equal("unix:/var/run/nginx/*.cafe.example.com-800.sock"))
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
