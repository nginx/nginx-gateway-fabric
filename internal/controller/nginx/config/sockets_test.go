package config

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetSocketNameTLS(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	g.Expect(getSocketNameTLS(800, "*.cafe.example.com")).To(Equal(
		fmt.Sprintf("%s*.cafe.example.com-800.sock", SocketBasePath),
	))
	g.Expect(getSocketNameTLS(8443, "")).To(Equal(fmt.Sprintf("%s8443.sock", SocketBasePath)))
}

func TestGetSocketNameTLSTerminate(t *testing.T) {
	t.Parallel()
	g := NewGomegaWithT(t)

	g.Expect(getSocketNameTLSTerminate(800, "*.cafe.example.com")).To(
		Equal(fmt.Sprintf("%s*.cafe.example.com-800-terminate.sock", SocketBasePath)),
	)
	g.Expect(getSocketNameTLSTerminate(8443, "")).To(Equal(fmt.Sprintf("%s8443-terminate.sock", SocketBasePath)))
}

func TestGetSocketNameHTTPS(t *testing.T) {
	t.Parallel()
	res := getSocketNameHTTPS(800)

	g := NewGomegaWithT(t)
	g.Expect(res).To(Equal(fmt.Sprintf("%shttps800.sock", SocketBasePath)))
}

func TestGetTLSPassthroughVarName(t *testing.T) {
	t.Parallel()
	res := getTLSPassthroughVarName(800)

	g := NewGomegaWithT(t)
	g.Expect(res).To(Equal("$dest800"))
}
