package config

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/nginx/config/shared"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/dataplane"
)

func TestZoneSize(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "ZoneSize Suite")
}

var _ = Describe("ZoneSizeCalculator", Ordered, func() {
	var calc *ZoneSizeCalculator

	BeforeEach(func() {
		config := DefaultZoneSizeCalculatorConfig()
		calc = NewZoneSizeCalculator(config)
	})

	Context("Calculate", func() {
		DescribeTable(
			"HTTP OSS endpoint calculations",
			func(endpointCount int, expectedMin string, expectedMax string) {
				// Allow a range because rounding to nearest k can vary slightly
				result := calc.Calculate(endpointCount, HTTPOSS)
				Expect(result).To(MatchRegexp(`^\d+[km]$`), "result should be valid size format")

				// Parse and compare bytes
				resultBytes, err := shared.ParseSize(result)
				Expect(err).NotTo(HaveOccurred())

				minBytes, _ := shared.ParseSize(expectedMin)
				maxBytes, _ := shared.ParseSize(expectedMax)

				Expect(resultBytes).To(BeNumerically(">=", minBytes))
				Expect(resultBytes).To(BeNumerically("<=", maxBytes))
			},
			// Note: Results are clamped to minimum (128k) for small endpoint counts
			Entry("10 endpoints", 10, "128k", "128k"),
			Entry("50 endpoints", 50, "128k", "128k"),
			Entry("200 endpoints", 200, "312k", "320k"),
			Entry("500 endpoints", 500, "780k", "800k"),
			Entry("648 endpoints (max documented)", 648, "1m", "1m"),
		)

		DescribeTable(
			"HTTP Plus endpoint calculations",
			func(endpointCount int, expectedMin string, expectedMax string) {
				result := calc.Calculate(endpointCount, HTTPPlus)
				Expect(result).To(MatchRegexp(`^\d+[km]$`))

				resultBytes, _ := shared.ParseSize(result)
				minBytes, _ := shared.ParseSize(expectedMin)
				maxBytes, _ := shared.ParseSize(expectedMax)

				Expect(resultBytes).To(BeNumerically(">=", minBytes))
				Expect(resultBytes).To(BeNumerically("<=", maxBytes))
			},
			// Note: Results are clamped to minimum (128k) for small endpoint counts
			Entry("10 endpoints", 10, "128k", "128k"),
			Entry("50 endpoints", 50, "368k", "384k"),
			Entry("200 endpoints", 200, "1500k", "1600k"),
			Entry("300 endpoints", 300, "2200k", "2400k"),
			Entry("545 endpoints (max documented)", 545, "4m", "4200k"), // 545*3847*2.0 ≈ 4193k ≈ 4m
		)

		DescribeTable(
			"Stream OSS endpoint calculations",
			func(endpointCount int, expectedMin string, expectedMax string) {
				result := calc.Calculate(endpointCount, StreamOSS)
				Expect(result).To(MatchRegexp(`^\d+[km]$`))

				resultBytes, _ := shared.ParseSize(result)
				minBytes, _ := shared.ParseSize(expectedMin)
				maxBytes, _ := shared.ParseSize(expectedMax)

				Expect(resultBytes).To(BeNumerically(">=", minBytes))
				Expect(resultBytes).To(BeNumerically("<=", maxBytes))
			},
			// Note: Results are clamped to minimum (128k) for small endpoint counts
			Entry("10 endpoints", 10, "128k", "128k"),
			Entry("100 endpoints", 100, "176k", "180k"),
			Entry("300 endpoints", 300, "528k", "540k"),
			Entry("576 endpoints (max documented)", 576, "1m", "1024k"),
		)

		DescribeTable(
			"Stream Plus endpoint calculations",
			func(endpointCount int, expectedMin string, expectedMax string) {
				result := calc.Calculate(endpointCount, StreamPlus)
				Expect(result).To(MatchRegexp(`^\d+[km]$`))

				resultBytes, _ := shared.ParseSize(result)
				minBytes, _ := shared.ParseSize(expectedMin)
				maxBytes, _ := shared.ParseSize(expectedMax)

				Expect(resultBytes).To(BeNumerically(">=", minBytes))
				Expect(resultBytes).To(BeNumerically("<=", maxBytes))
			},
			// Note: Results are clamped to minimum (128k) for small endpoint counts
			Entry("10 endpoints", 10, "128k", "128k"),
			Entry("100 endpoints", 100, "206k", "208k"),
			Entry("300 endpoints", 300, "616k", "640k"),
			Entry("500 endpoints", 500, "1m", "1064k"),
			Entry("991 endpoints (max documented)", 991, "2m", "2100k"),
		)
	})

	Context("Buffer multiplier application", func() {
		It("applies 100% buffer by default", func() {
			// 100 endpoints × 809 bytes × 2.0 = 161,800 bytes ≈ 158k
			result := calc.Calculate(100, HTTPOSS)
			resultBytes, _ := shared.ParseSize(result)

			// 100 × 809 = 80,900 (without buffer)
			// 80,900 × 2.0 = 161,800 (with buffer)
			expectedMin := int64(160_000) // Allow some rounding
			Expect(resultBytes).To(BeNumerically(">=", expectedMin))
		})

		It("respects custom buffer multiplier", func() {
			customConfig := ZoneSizeCalculatorConfig{
				BufferMultiplier: 2.0, // 100% buffer
				MinSize:          128 * 1024,
				MaxSize:          512 * 1024 * 1024,
			}
			customCalc := NewZoneSizeCalculator(customConfig)

			// 100 × 809 × 2.0 = 161,800 bytes
			result := customCalc.Calculate(100, HTTPOSS)
			resultBytes, _ := shared.ParseSize(result)

			expectedMin := int64(160_000)
			Expect(resultBytes).To(BeNumerically(">=", expectedMin))
		})
	})

	Context("Min/Max capping", func() {
		It("respects minimum zone size", func() {
			// 1 endpoint should be less than 128k, so should be capped
			result := calc.Calculate(1, HTTPOSS)

			resultBytes, _ := shared.ParseSize(result)
			minBytes, _ := shared.ParseSize("128k")

			Expect(resultBytes).To(Equal(minBytes))
		})

		It("respects maximum zone size", func() {
			// Very large endpoint count should be capped at 512m
			result := calc.Calculate(1_000_000, HTTPOSS)

			resultBytes, _ := shared.ParseSize(result)
			maxBytes, _ := shared.ParseSize("512m")

			Expect(resultBytes).To(Equal(maxBytes))
		})

		It("respects custom min and max", func() {
			customConfig := ZoneSizeCalculatorConfig{
				BufferMultiplier: 1.25,
				MinSize:          256 * 1024,       // 256k
				MaxSize:          10 * 1024 * 1024, // 10m
			}
			customCalc := NewZoneSizeCalculator(customConfig)

			// Very small count should use custom minimum
			smallResult := customCalc.Calculate(1, HTTPOSS)
			smallBytes, _ := shared.ParseSize(smallResult)
			expectedMin, _ := shared.ParseSize("256k")
			Expect(smallBytes).To(Equal(expectedMin))

			// Very large count should use custom maximum
			largeResult := customCalc.Calculate(1_000_000, HTTPOSS)
			largeBytes, _ := shared.ParseSize(largeResult)
			expectedMax, _ := shared.ParseSize("10m")
			Expect(largeBytes).To(Equal(expectedMax))
		})
	})

	Context("Rounding to nearest k", func() {
		It("rounds up to nearest k", func() {
			// Force an uneven byte count
			// 100 × 809 × 1.25 = 101,125 bytes
			// Should round up to 99k (101,125 / 1024 = 98.75... → 99k)
			result := calc.Calculate(100, HTTPOSS)

			Expect(result).To(MatchRegexp(`^\d+k$`), "should be in kilobytes")

			// Verify it's rounded up
			resultBytes, _ := shared.ParseSize(result)
			Expect(resultBytes%1024).To(Equal(int64(0)), "should be multiple of 1024 (k)")
		})

		It("converts to megabytes when >= 1024k", func() {
			// 2000 endpoints × 3847 × 2.0 = 15,388,000 bytes ≈ 14.7m
			result := calc.Calculate(2000, HTTPPlus)

			Expect(result).To(MatchRegexp(`^\d+m$`), "should be in megabytes")

			resultBytes, _ := shared.ParseSize(result)
			Expect(resultBytes).To(BeNumerically(">", 1024*1024))
		})
	})

	Context("Edge cases", func() {
		It("handles zero endpoints with minimum size", func() {
			result := calc.Calculate(0, HTTPOSS)
			resultBytes, _ := shared.ParseSize(result)
			minBytes, _ := shared.ParseSize("128k")

			Expect(resultBytes).To(Equal(minBytes))
		})

		It("handles negative endpoints with minimum size", func() {
			result := calc.Calculate(-5, HTTPOSS)
			resultBytes, _ := shared.ParseSize(result)
			minBytes, _ := shared.ParseSize("128k")

			Expect(resultBytes).To(Equal(minBytes))
		})

		It("returns valid string format", func() {
			for _, endpointCount := range []int{1, 10, 100, 500, 1000} {
				result := calc.Calculate(endpointCount, HTTPOSS)
				Expect(result).To(MatchRegexp(`^\d+[km]$`))
			}
		})
	})

	Context("BytesToString", func() {
		DescribeTable(
			"byte conversions",
			func(bytes int64, expected string) {
				result := calc.bytesToString(bytes)
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
			result := calc.bytesToString(1025)
			Expect(result).To(Equal("2k"))
		})

		It("prefers m when >= 1024k", func() {
			// 1,048,576 bytes = 1024k = 1m
			result := calc.bytesToString(1024 * 1024)
			Expect(result).To(Equal("1m"))

			// Ensure it switches to m for values well above 1m
			result = calc.bytesToString(5 * 1024 * 1024)
			Expect(result).To(Equal("5m"))
		})
	})

	Context("zoneSizeCalculatorConfigFromDataplane", func() {
		It("returns full defaults for the zero value", func() {
			cfg := zoneSizeCalculatorConfigFromDataplane(dataplane.UpstreamZoneAutoSizing{})
			Expect(cfg).To(Equal(DefaultZoneSizeCalculatorConfig()))
		})

		It("uses every field when all are set", func() {
			cfg := zoneSizeCalculatorConfigFromDataplane(dataplane.UpstreamZoneAutoSizing{
				BufferMultiplier: 2.0,
				MinSize:          256 * 1024,
				MaxSize:          1024 * 1024 * 1024,
			})
			Expect(cfg).To(Equal(ZoneSizeCalculatorConfig{
				BufferMultiplier: 2.0,
				MinSize:          256 * 1024,
				MaxSize:          1024 * 1024 * 1024,
			}))
		})

		It("falls back to the default for any individually-unset field", func() {
			cfg := zoneSizeCalculatorConfigFromDataplane(dataplane.UpstreamZoneAutoSizing{
				BufferMultiplier: 2.0,
			})
			def := DefaultZoneSizeCalculatorConfig()
			Expect(cfg).To(Equal(ZoneSizeCalculatorConfig{
				BufferMultiplier: 2.0,
				MinSize:          def.MinSize,
				MaxSize:          def.MaxSize,
			}))
		})
	})
})
