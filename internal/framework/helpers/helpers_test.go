package helpers_test

import (
	"testing"
	"text/template"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestMustCastObject(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var obj client.Object = &gatewayv1.Gateway{}

	g.Expect(func() {
		_ = helpers.MustCastObject[*gatewayv1.Gateway](obj)
	}).ToNot(Panic())

	g.Expect(func() {
		_ = helpers.MustCastObject[*gatewayv1.BackendTLSPolicy](obj)
	}).To(Panic())
}

func TestEqualPointers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		p1       *string
		p2       *string
		name     string
		expEqual bool
	}{
		{
			name:     "first pointer nil; second has non-empty value",
			p1:       nil,
			p2:       helpers.GetPointer("test"),
			expEqual: false,
		},
		{
			name:     "second pointer nil; first has non-empty value",
			p1:       helpers.GetPointer("test"),
			p2:       nil,
			expEqual: false,
		},
		{
			name:     "different values",
			p1:       helpers.GetPointer("test"),
			p2:       helpers.GetPointer("different"),
			expEqual: false,
		},
		{
			name:     "both pointers nil",
			p1:       nil,
			p2:       nil,
			expEqual: true,
		},
		{
			name:     "first pointer nil; second empty",
			p1:       nil,
			p2:       helpers.GetPointer(""),
			expEqual: true,
		},
		{
			name:     "second pointer nil; first empty",
			p1:       helpers.GetPointer(""),
			p2:       nil,
			expEqual: true,
		},
		{
			name:     "same value",
			p1:       helpers.GetPointer("test"),
			p2:       helpers.GetPointer("test"),
			expEqual: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			val := helpers.EqualPointers(test.p1, test.p2)
			g.Expect(val).To(Equal(test.expEqual))
		})
	}
}

func TestMustExecuteTemplate(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	tmpl := template.Must(template.New("test").Parse(`Hello {{.}}`))
	bytes := helpers.MustExecuteTemplate(tmpl, "you")
	g.Expect(string(bytes)).To(Equal("Hello you"))
}

func TestMustExecuteTemplatePanics(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	execute := func() {
		helpers.MustExecuteTemplate(nil, nil)
	}

	g.Expect(execute).To(Panic())
}

func TestCapitalizeString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "empty string",
			in:   "",
			out:  "",
		},
		{
			name: "single lowercase letter",
			in:   "a",
			out:  "A",
		},
		{
			name: "lowercase word",
			in:   "gateway",
			out:  "Gateway",
		},
		{
			name: "Phrase with mixed case",
			in:   "gateway API not found",
			out:  "Gateway API not found",
		},
		{
			name: "non-letter first char",
			in:   "1abc",
			out:  "1abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(helpers.CapitalizeString(tc.in)).To(Equal(tc.out))
		})
	}
}

func TestBuildPortFwdURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		scheme      string
		url         string
		path        string
		expectedURL string
		port        int
	}{
		{
			name:        "Test port without a path",
			scheme:      "http",
			url:         "cafe.example.com",
			path:        "",
			expectedURL: "http://cafe.example.com:80",
			port:        80,
		},
		{
			name:        "Test coffee path",
			scheme:      "http",
			url:         "cafe.example.com",
			path:        "coffee",
			expectedURL: "http://cafe.example.com:80/coffee",
			port:        80,
		},
		{
			name:        "Test tea path",
			scheme:      "http",
			url:         "cafe.example.com",
			path:        "tea",
			expectedURL: "http://cafe.example.com:80/tea",
			port:        80,
		},
		{
			name:        "Test non-privileged port, without path",
			scheme:      "http",
			url:         "cafe.example.com",
			path:        "",
			expectedURL: "http://cafe.example.com:8080",
			port:        8080,
		},
		{
			name:        "Test non-privileged port, with path",
			scheme:      "http",
			url:         "cafe.example.com",
			path:        "coffee",
			expectedURL: "http://cafe.example.com:8080/coffee",
			port:        8080,
		},
		{
			name:        "Test omit port",
			scheme:      "http",
			url:         "cafe.example.com",
			path:        "tea",
			expectedURL: "http://cafe.example.com/tea",
			port:        0,
		},
		{
			name:        "Test omit scheme",
			scheme:      "",
			url:         "cafe.example.com",
			path:        "",
			expectedURL: "http://cafe.example.com",
			port:        0,
		},
		{
			name:        "Test https scheme",
			scheme:      "https",
			url:         "cafe.example.com",
			path:        "",
			expectedURL: "https://cafe.example.com",
			port:        0,
		},
		{
			name:        "Test https scheme on port 443",
			scheme:      "https",
			url:         "cafe.example.com",
			path:        "",
			expectedURL: "https://cafe.example.com:443",
			port:        443,
		},
		{
			name:        "Test preserve query",
			scheme:      "",
			url:         "cafe.example.com",
			path:        "coffee?x=%%3C%%2Fscript%%3E",
			expectedURL: "http://cafe.example.com:80/coffee?x=%%3C%%2Fscript%%3E",
			port:        80,
		},
		{
			name:        "Test preserve fragment",
			scheme:      "",
			url:         "cafe.example.com",
			path:        "#menu",
			expectedURL: "http://cafe.example.com#menu",
			port:        0,
		},
		{
			name:        "Test preserve query and fragment",
			scheme:      "",
			url:         "cafe.example.com",
			path:        "coffee?x=%%3C%%2Fscript%%3E#menu",
			expectedURL: "http://cafe.example.com/coffee?x=%%3C%%2Fscript%%3E#menu",
			port:        0,
		},
		{
			name:        "Test bracketed IPv6",
			scheme:      "",
			url:         "::1",
			path:        "hello",
			expectedURL: "http://[::1]:443/hello",
			port:        443,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(helpers.BuildPortFwdURL(tt.scheme, tt.url, tt.port, tt.path)).To(Equal(tt.expectedURL))
		})
	}
}
