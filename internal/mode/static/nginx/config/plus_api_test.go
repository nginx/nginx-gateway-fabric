package config

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/internal/mode/static/state/dataplane"
)

func TestExecutePlusAPI(t *testing.T) {
	t.Parallel()
	conf := dataplane.Configuration{
		NginxPlus: dataplane.NginxPlus{AllowedAddresses: []string{"127.0.0.1", "25.0.0.3"}},
	}

	g := NewWithT(t)
	expSubStrings := map[string]int{
		"listen unix:/var/run/nginx/nginx-plus-api.sock;": 1,
		"access_log off;":               2,
		"api write=on;":                 1,
		"listen 8765;":                  1,
		"root /usr/share/nginx/html;":   1,
		"allow 127.0.0.1;":              1,
		"allow 25.0.0.3;":               1,
		"deny all;":                     1,
		"location = /dashboard.html {}": 1,
		"api write=off;":                1,
	}

	for expSubStr, expCount := range expSubStrings {
		res := executePlusAPI(conf)
		g.Expect(res).To(HaveLen(1))
		g.Expect(expCount).To(Equal(strings.Count(string(res[0].data), expSubStr)))
	}
}
