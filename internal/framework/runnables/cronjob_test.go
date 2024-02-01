package runnables

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestCronJob(t *testing.T) {
	g := NewWithT(t)

	timeout := 10 * time.Second
	var callCount int

	valCh := make(chan int, 128)
	worker := func(context.Context) {
		callCount++
		valCh <- callCount
	}

	cfg := CronJobConfig{
		Worker: worker,
		Logger: zap.New(),
		Period: 1 * time.Millisecond, // 1ms is much smaller than timeout so the CronJob should run a few times
	}
	job := NewCronJob(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	errCh := make(chan error)
	go func() {
		errCh <- job.Start(ctx)
		close(errCh)
	}()

	minReports := 2 // ensure that the CronJob reports more than once: it doesn't exit after the first run

	g.Eventually(valCh).Should(Receive(BeNumerically(">=", minReports)))

	cancel()
	g.Eventually(errCh).Should(Receive(BeNil()))
	g.Eventually(errCh).Should(BeClosed())
}
