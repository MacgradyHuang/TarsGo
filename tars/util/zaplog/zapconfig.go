package zaplog

import (
	"os"
	"path"
)

type zapLoggerConf struct {
	isTestEnv   bool
	processName string
	withPid     bool
	logApiPath  string
	listenAddr  string
	hostName    string
	eLKTempName string
	logPath     string
}

var defaultOptions = zapLoggerConf{
	isTestEnv:   true,
	processName: path.Base(os.Args[0]),
	withPid:     true,
	logApiPath:  "/log",
	listenAddr:  "127.0.0.1:0",
	eLKTempName: path.Base(os.Args[0]),
}

type zapLoggerOption interface {
	apply(*zapLoggerConf)
}

type zapLoggerOptionFunc func(*zapLoggerConf)

func (t zapLoggerOptionFunc) apply(option *zapLoggerConf) {
	t(option)
}

func IsTestEnv(isTestEnv bool) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.isTestEnv = isTestEnv
	})
}

func ProcessName(processName string) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.processName = processName
	})
}

func WithPid(withPid bool) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.withPid = withPid
	})
}

func LogApiPath(logApiPath string) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.logApiPath = logApiPath
	})
}

func ListenAddr(addr string) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.listenAddr = addr
	})
}

func HostName(hostname string) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.hostName = hostname
	})
}

func ELKTempName(eLKTempName string) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.eLKTempName = eLKTempName
	})
}

func LogPath(path string) zapLoggerOption {
	return zapLoggerOptionFunc(func(option *zapLoggerConf) {
		option.logPath = path
	})
}
