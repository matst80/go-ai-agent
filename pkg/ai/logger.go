package ai

// Logger is a minimal logging interface used by truncators and other ai package
// components. Consumers may provide their own implementation (zap, logrus, etc.).
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// NoopLogger implements Logger but does nothing. Useful as the default.
type NoopLogger struct{}

func (NoopLogger) Debugf(format string, args ...any) {}
func (NoopLogger) Infof(format string, args ...any)  {}
func (NoopLogger) Warnf(format string, args ...any)  {}
func (NoopLogger) Errorf(format string, args ...any) {}
