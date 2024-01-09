package static

import (
	"errors"

	"github.com/go-kit/log"
	"github.com/prometheus/common/promlog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . logLevelSetter

// logLevelSetter defines an interface for setting the logging level of a logger.
type logLevelSetter interface {
	SetLevel(string) error
}

// multiLogLevelSetter sets the log level for multiple logLevelSetters.
type multiLogLevelSetter struct {
	setters []logLevelSetter
}

func newMultiLogLevelSetter(setters ...logLevelSetter) multiLogLevelSetter {
	return multiLogLevelSetter{setters: setters}
}

func (m multiLogLevelSetter) SetLevel(level string) error {
	allErrs := make([]error, 0, len(m.setters))

	for _, s := range m.setters {
		if err := s.SetLevel(level); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return errors.Join(allErrs...)
}

// zapLogLevelSetter sets the level for a zap logger.
type zapLogLevelSetter struct {
	atomicLevel zap.AtomicLevel
}

func newZapLogLevelSetter(atomicLevel zap.AtomicLevel) zapLogLevelSetter {
	return zapLogLevelSetter{
		atomicLevel: atomicLevel,
	}
}

// SetLevel sets the logging level for the zap logger.
func (z zapLogLevelSetter) SetLevel(level string) error {
	parsedLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return err
	}
	z.atomicLevel.SetLevel(parsedLevel)

	return nil
}

// Enabled returns true if the given level is at or above the current level.
func (z zapLogLevelSetter) Enabled(level zapcore.Level) bool {
	return z.atomicLevel.Enabled(level)
}

// leveledPrometheusLogger is a leveled prometheus logger.
// This interface is required because the promlog.NewDynamic returns an unexported type *logger.
type leveledPrometheusLogger interface {
	log.Logger
	SetLevel(level *promlog.AllowedLevel)
}

type promLogLevelSetter struct {
	logger leveledPrometheusLogger
}

func newPromLogLevelSetter(logger leveledPrometheusLogger) promLogLevelSetter {
	return promLogLevelSetter{logger: logger}
}

func newLeveledPrometheusLogger() (leveledPrometheusLogger, error) {
	logFormat := &promlog.AllowedFormat{}

	if err := logFormat.Set("json"); err != nil {
		return nil, err
	}

	logConfig := &promlog.Config{Format: logFormat}
	logger := promlog.NewDynamic(logConfig)

	return logger, nil
}

func (p promLogLevelSetter) SetLevel(level string) error {
	al := &promlog.AllowedLevel{}
	if err := al.Set(level); err != nil {
		return err
	}

	p.logger.SetLevel(al)
	return nil
}
