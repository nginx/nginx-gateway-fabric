package framework

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

type Target struct {
	Header http.Header
	Method string
	URL    string
	Body   []byte
}

func convertTargetToVegetaTarget(targets []Target) []vegeta.Target {
	vegTargets := make([]vegeta.Target, 0, len(targets))
	for _, t := range targets {
		vt := vegeta.Target{
			Method: t.Method,
			URL:    t.URL,
			Body:   t.Body,
			Header: t.Header,
		}
		vegTargets = append(vegTargets, vt)
	}
	return vegTargets
}

// LoadTestConfig is the configuration to run a load test.
type LoadTestConfig struct {
	Description string
	Proxy       string
	ServerName  string
	Targets     []Target
	Rate        int
	Duration    time.Duration
}

// Metrics is a wrapper around the vegeta Metrics.
type Metrics struct {
	vegeta.Metrics
}

// RunLoadTest uses Vegeta to send traffic to the provided Targets at the given rate for the given duration and writes
// the results to the provided file.
func RunLoadTest(cfg LoadTestConfig) (vegeta.Results, Metrics) {
	GinkgoWriter.Printf("Running load test: %s\n", cfg.Description)
	vegTargets := convertTargetToVegetaTarget(cfg.Targets)
	targeter := vegeta.NewStaticTargeter(vegTargets...)

	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{IP: vegeta.DefaultLocalAddr.IP, Zone: vegeta.DefaultLocalAddr.Zone},
		KeepAlive: 30 * time.Second,
	}

	httpClient := http.Client{
		Timeout: vegeta.DefaultTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, network, cfg.Proxy)
				return conn, err
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // self-signed cert for testing
				ServerName:         cfg.ServerName,
			},
			MaxIdleConnsPerHost: vegeta.DefaultConnections,
			MaxConnsPerHost:     vegeta.DefaultMaxConnections,
		},
	}

	attacker := vegeta.NewAttacker(vegeta.Client(&httpClient))

	r := vegeta.Rate{Freq: cfg.Rate, Per: time.Second}
	var results vegeta.Results
	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, r, cfg.Duration, cfg.Description) {
		results = append(results, *res)
		metrics.Add(res)
	}
	metrics.Close()

	return results, Metrics{metrics}
}
