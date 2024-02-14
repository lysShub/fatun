package control

import (
	"log"
	"os"
)

type ZapLogger struct {
	logger log.Logger
}

// NewZapLogger 创建封装了zap的对象，该对象是对LoggerV2接口的实现
func NewZapLogger() *ZapLogger {
	return &ZapLogger{
		logger: *log.New(os.Stdout, "", 0),
	}
}

func isCore(v any) bool {
	if v, ok := v.(string); ok {
		return v == "[core]"
	}
	return false
}

// Info returns
func (zl *ZapLogger) Info(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Infoln returns
func (zl *ZapLogger) Infoln(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Infof returns
func (zl *ZapLogger) Infof(format string, args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Printf(format, args...)
}

// Warning returns
func (zl *ZapLogger) Warning(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Warningln returns
func (zl *ZapLogger) Warningln(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Warningf returns
func (zl *ZapLogger) Warningf(format string, args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Printf(format, args...)
}

// Error returns
func (zl *ZapLogger) Error(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Errorln returns
func (zl *ZapLogger) Errorln(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Errorf returns
func (zl *ZapLogger) Errorf(format string, args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Printf(format, args...)
}

// Fatal returns
func (zl *ZapLogger) Fatal(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Fatalln returns
func (zl *ZapLogger) Fatalln(args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Println(args...)
}

// Fatalf logs to fatal level
func (zl *ZapLogger) Fatalf(format string, args ...interface{}) {
	if isCore(args[0]) {
		return
	}
	zl.logger.Printf(format, args...)
}

// V reports whether verbosity level l is at least the requested verbose level.
func (zl *ZapLogger) V(v int) bool {
	return false
}
