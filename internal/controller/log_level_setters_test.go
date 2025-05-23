package controller

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/nginx/nginx-gateway-fabric/internal/controller/controllerfakes"
)

func TestMultiLogLevelSetter_SetLevel(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	logr1 := &controllerfakes.FakeLogLevelSetter{}
	logr2 := &controllerfakes.FakeLogLevelSetter{}
	logr3 := &controllerfakes.FakeLogLevelSetter{}

	multiSetter := newMultiLogLevelSetter(logr1, logr2, logr3)
	g.Expect(multiSetter.SetLevel("test")).To(Succeed())

	g.Expect(logr1.SetLevelCallCount()).To(Equal(1))
	g.Expect(logr2.SetLevelCallCount()).To(Equal(1))
	g.Expect(logr3.SetLevelCallCount()).To(Equal(1))

	// error case
	err1 := errors.New("error1")
	err2 := errors.New("error2")
	err3 := errors.New("error3")

	logr1.SetLevelReturns(err1)
	logr2.SetLevelReturns(err2)
	logr3.SetLevelReturns(err3)

	err := multiSetter.SetLevel("test")
	g.Expect(err).To(HaveOccurred())

	expErr := errors.Join(err1, err2, err3)
	g.Expect(err).To(MatchError(expErr))
}

func TestZapLogLevelSetter_SetLevel(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	zapSetter := newZapLogLevelSetter(zap.NewAtomicLevel())

	g.Expect(zapSetter.SetLevel("error")).To(Succeed())
	g.Expect(zapSetter.Enabled(zap.ErrorLevel)).To(BeTrue())

	g.Expect(zapSetter.SetLevel("info")).To(Succeed())
	g.Expect(zapSetter.Enabled(zap.InfoLevel)).To(BeTrue())

	g.Expect(zapSetter.SetLevel("debug")).To(Succeed())
	g.Expect(zapSetter.Enabled(zap.DebugLevel)).To(BeTrue())

	g.Expect(zapSetter.SetLevel("invalid")).ToNot(Succeed())
}
