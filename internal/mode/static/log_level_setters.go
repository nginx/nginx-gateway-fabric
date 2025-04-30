package static

import (
	"errors"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

//counterfeiter:generate . logLevelSetter

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
