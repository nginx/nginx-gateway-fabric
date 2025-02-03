package config

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

func TestExecuteVersion(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	conf := dataplane.Configuration{Version: 42}
	res := executeVersion(conf)
	g.Expect(res).To(HaveLen(1))
	g.Expect(res[0].dest).To(Equal(configVersionFile))
	g.Expect(string(res[0].data)).To(ContainSubstring("return 200 42;"))
}
