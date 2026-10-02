package config

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
)

func TestZoneSize(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "ZoneSize Suite")
}

func strPtr(s string) *string { return &s }

var _ = Describe("ZoneSizeCalculator", func() {
	var calc *ZoneSizeCalculator

	BeforeEach(func() {
		calc = NewZoneSizeCalculator(nil, shared.DefaultZoneSizeMaxSize)
	})

	Context("Resolve", func() {
		DescribeTable(
			"per-profile static defaults for nil (unset) explicit, when no override exists",
			func(explicit *string, profile ZoneSizeProfile, expected string) {
				result := calc.Resolve("my-upstream", explicit, profile)
				Expect(result).To(Equal(expected))
			},
			Entry("nil (unset) HTTP OSS", nil, HTTPOSS, "512k"),
			Entry("nil (unset) HTTP Plus", nil, HTTPPlus, "2m"),
			Entry("nil (unset) Stream OSS", nil, StreamOSS, "512k"),
			Entry("nil (unset) Stream Plus", nil, StreamPlus, "1m"),
		)

		DescribeTable(
			"flat 64k cold start for explicit \"auto\", regardless of profile, when no override exists",
			func(explicit *string, profile ZoneSizeProfile, expected string) {
				result := calc.Resolve("my-upstream", explicit, profile)
				Expect(result).To(Equal(expected))
			},
			Entry(`"auto" HTTP OSS`, strPtr("auto"), HTTPOSS, "64k"),
			Entry(`"auto" HTTP Plus`, strPtr("auto"), HTTPPlus, "64k"),
			Entry(`"auto" Stream OSS`, strPtr("auto"), StreamOSS, "64k"),
			Entry(`"auto" Stream Plus`, strPtr("auto"), StreamPlus, "64k"),
		)

		It("resolves nil and \"auto\" to different sizes when no override exists (nil stays at the "+
			"static per-profile default; auto cold-starts at a flat 64k)", func() {
			Expect(calc.Resolve("my-upstream", nil, HTTPPlus)).To(Equal("2m"))
			Expect(calc.Resolve("my-upstream", strPtr("auto"), HTTPPlus)).To(Equal("64k"))
		})

		It("returns an explicit static size verbatim, ignoring the profile default", func() {
			result := calc.Resolve("my-upstream", strPtr("10m"), HTTPOSS)
			Expect(result).To(Equal("10m"))
		})

		It("ignores any override for a nil (unset) explicit, always using the static default", func() {
			overrides := map[string]int64{"my-upstream": 4 * 1024 * 1024}
			calc = NewZoneSizeCalculator(overrides, shared.DefaultZoneSizeMaxSize)

			result := calc.Resolve("my-upstream", nil, HTTPOSS)
			Expect(result).To(Equal("512k"))
		})

		It("uses the override for an upstream when one exists, for explicit auto", func() {
			overrides := map[string]int64{"my-upstream": 4 * 1024 * 1024}
			calc = NewZoneSizeCalculator(overrides, shared.DefaultZoneSizeMaxSize)

			result := calc.Resolve("my-upstream", strPtr("auto"), HTTPOSS)
			Expect(result).To(Equal("4m"))
		})

		It("does not apply an override to an upstream with an explicit static size", func() {
			overrides := map[string]int64{"my-upstream": 4 * 1024 * 1024}
			calc = NewZoneSizeCalculator(overrides, shared.DefaultZoneSizeMaxSize)

			result := calc.Resolve("my-upstream", strPtr("10m"), HTTPOSS)
			Expect(result).To(Equal("10m"))
		})

		It("does not apply another upstream's override", func() {
			overrides := map[string]int64{"other-upstream": 4 * 1024 * 1024}
			calc = NewZoneSizeCalculator(overrides, shared.DefaultZoneSizeMaxSize)

			result := calc.Resolve("my-upstream", nil, HTTPOSS)
			Expect(result).To(Equal("512k"))
		})
	})

	Context("IsAuto", func() {
		It("is false for nil (unset requires an explicit opt-in to auto-sizing)", func() {
			Expect(IsAuto(nil)).To(BeFalse())
		})

		It(`is true for "auto"`, func() {
			Expect(IsAuto(strPtr("auto"))).To(BeTrue())
		})

		It("is false for an explicit static size", func() {
			Expect(IsAuto(strPtr("10m"))).To(BeFalse())
		})
	})

	Context("CurrentSizeBytes", func() {
		It("returns the flat auto-sizing cold start when no override exists", func() {
			Expect(calc.CurrentSizeBytes("my-upstream")).To(Equal(int64(64 * 1024)))
		})

		It("returns the override when one exists", func() {
			overrides := map[string]int64{"my-upstream": 4 * 1024 * 1024}
			calc = NewZoneSizeCalculator(overrides, shared.DefaultZoneSizeMaxSize)

			Expect(calc.CurrentSizeBytes("my-upstream")).To(Equal(int64(4 * 1024 * 1024)))
		})
	})

	Context("NextSize", func() {
		It("doubles the current size", func() {
			next, ok := calc.NextSize(1024 * 1024)
			Expect(ok).To(BeTrue())
			Expect(next).To(Equal(int64(2 * 1024 * 1024)))
		})

		It("caps growth at maxSize", func() {
			calc = NewZoneSizeCalculator(nil, 3*1024*1024)

			next, ok := calc.NextSize(2 * 1024 * 1024)
			Expect(ok).To(BeTrue())
			Expect(next).To(Equal(int64(3 * 1024 * 1024)))
		})

		It("returns false when already at maxSize", func() {
			calc = NewZoneSizeCalculator(nil, 2*1024*1024)

			next, ok := calc.NextSize(2 * 1024 * 1024)
			Expect(ok).To(BeFalse())
			Expect(next).To(Equal(int64(2 * 1024 * 1024)))
		})

		It("returns false when already above maxSize", func() {
			calc = NewZoneSizeCalculator(nil, 1024*1024)

			next, ok := calc.NextSize(2 * 1024 * 1024)
			Expect(ok).To(BeFalse())
			Expect(next).To(Equal(int64(2 * 1024 * 1024)))
		})
	})

	Context("PrevSize", func() {
		It("halves the current size", func() {
			prev, ok := calc.PrevSize(4 * 1024 * 1024)
			Expect(ok).To(BeTrue())
			Expect(prev).To(Equal(int64(2 * 1024 * 1024)))
		})

		It("returns false when already at floor", func() {
			prev, ok := calc.PrevSize(64 * 1024)
			Expect(ok).To(BeFalse())
			Expect(prev).To(Equal(int64(64 * 1024)))
		})

		It("returns false when already below floor", func() {
			prev, ok := calc.PrevSize(32 * 1024)
			Expect(ok).To(BeFalse())
			Expect(prev).To(Equal(int64(32 * 1024)))
		})
	})

	Context("HTTPProfile / StreamProfile", func() {
		It("selects the correct profile for OSS and Plus", func() {
			Expect(HTTPProfile(false)).To(Equal(HTTPOSS))
			Expect(HTTPProfile(true)).To(Equal(HTTPPlus))
			Expect(StreamProfile(false)).To(Equal(StreamOSS))
			Expect(StreamProfile(true)).To(Equal(StreamPlus))
		})
	})

	Context("bytesToString", func() {
		DescribeTable(
			"byte conversions",
			func(bytes int64, expected string) {
				result := bytesToString(bytes)
				Expect(result).To(Equal(expected))
			},
			Entry("256 bytes", int64(256), "1k"),
			Entry("256k bytes", int64(256*1024), "256k"),
			Entry("512k bytes", int64(512*1024), "512k"),
			Entry("1m bytes", int64(1024*1024), "1m"),
			Entry("2m bytes", int64(2*1024*1024), "2m"),
			Entry("512m bytes", int64(512*1024*1024), "512m"),
			Entry("1m + 512k bytes", int64(1.5*1024*1024), "2m"),
		)

		It("rounds up to nearest k", func() {
			// 1025 bytes should round up to 2k
			result := bytesToString(1025)
			Expect(result).To(Equal("2k"))
		})

		It("prefers m when >= 1024k", func() {
			// 1,048,576 bytes = 1024k = 1m
			result := bytesToString(1024 * 1024)
			Expect(result).To(Equal("1m"))

			// Ensure it switches to m for values well above 1m
			result = bytesToString(5 * 1024 * 1024)
			Expect(result).To(Equal("5m"))
		})
	})
})
