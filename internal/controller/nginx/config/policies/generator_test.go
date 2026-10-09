package policies_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/http"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/policies/policiesfakes"
)

var _ = Describe("Policy Generator", func() {
	Context("Composite Generator", func() {
		fakeGen1 := &policiesfakes.GeneratorMock{}
		fakeGen2 := &policiesfakes.GeneratorMock{}

		fakeGen1.GenerateForServerFunc = func([]policies.Policy, http.Server) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen1Server", Content: []byte("gen1Server-content")},
			}
		}
		fakeGen1.GenerateForStreamServerFunc = func([]policies.Policy) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen1StreamServer", Content: []byte("gen1StreamServer-content")},
			}
		}
		fakeGen1.GenerateForLocationFunc = func([]policies.Policy, http.Location) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen1Location", Content: []byte("gen1Location-content")},
			}
		}
		fakeGen1.GenerateForInternalLocationFunc = func([]policies.Policy) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen1IntLocation", Content: []byte("gen1IntLocation-content")},
			}
		}

		fakeGen2.GenerateForServerFunc = func([]policies.Policy, http.Server) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen2Server", Content: []byte("gen2Server-content")},
			}
		}
		fakeGen2.GenerateForStreamServerFunc = func([]policies.Policy) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen2StreamServer", Content: []byte("gen2StreamServer-content")},
			}
		}
		fakeGen2.GenerateForLocationFunc = func([]policies.Policy, http.Location) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen2Location", Content: []byte("gen2Location-content")},
			}
		}
		fakeGen2.GenerateForInternalLocationFunc = func([]policies.Policy) policies.GenerateResultFiles {
			return policies.GenerateResultFiles{
				{Name: "gen2IntLocation", Content: []byte("gen2IntLocation-content")},
			}
		}

		generator := policies.NewCompositeGenerator(fakeGen1, fakeGen2)

		It("returns proper server content", func() {
			expFiles := policies.GenerateResultFiles{
				{Name: "gen1Server", Content: []byte("gen1Server-content")},
				{Name: "gen2Server", Content: []byte("gen2Server-content")},
			}

			Expect(generator.GenerateForServer(nil, http.Server{})).To(BeEquivalentTo(expFiles))
		})

		It("returns proper location content", func() {
			expFiles := policies.GenerateResultFiles{
				{Name: "gen1Location", Content: []byte("gen1Location-content")},
				{Name: "gen2Location", Content: []byte("gen2Location-content")},
			}

			Expect(generator.GenerateForLocation(nil, http.Location{})).To(BeEquivalentTo(expFiles))
		})

		It("returns proper stream server content", func() {
			expFiles := policies.GenerateResultFiles{
				{Name: "gen1StreamServer", Content: []byte("gen1StreamServer-content")},
				{Name: "gen2StreamServer", Content: []byte("gen2StreamServer-content")},
			}

			Expect(generator.GenerateForStreamServer(nil)).To(BeEquivalentTo(expFiles))
		})

		It("returns proper internal location content", func() {
			expFiles := policies.GenerateResultFiles{
				{Name: "gen1IntLocation", Content: []byte("gen1IntLocation-content")},
				{Name: "gen2IntLocation", Content: []byte("gen2IntLocation-content")},
			}

			Expect(generator.GenerateForInternalLocation(nil)).To(BeEquivalentTo(expFiles))
		})
	})

	Context("Unimplemented Generator", func() {
		generator := policies.UnimplementedGenerator{}

		It("returns nil for GenerateForServer", func() {
			Expect(generator.GenerateForServer(nil, http.Server{})).To(BeNil())
		})

		It("returns nil for GenerateForLocation", func() {
			Expect(generator.GenerateForLocation(nil, http.Location{})).To(BeNil())
		})

		It("returns nil for GenerateForStreamServer", func() {
			Expect(generator.GenerateForStreamServer(nil)).To(BeNil())
		})

		It("returns nil for GenerateForInternalLocation", func() {
			Expect(generator.GenerateForInternalLocation(nil)).To(BeNil())
		})
	})
})
