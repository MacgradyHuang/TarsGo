package tars

import (
	"context"
	"errors"
	"fmt"
	"github.com/MacgradyHuang/TarsGo/tars/util/zaplog"
	"go.uber.org/zap"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MacgradyHuang/TarsGo/tars/util/debug"
)

// Admin struct
type Admin struct {
}

var (
	isShutdownByAdmin int32 = 0
)

// Shutdown shutdown all servant by admin
func (a *Admin) Shutdown() error {
	atomic.StoreInt32(&isShutdownByAdmin, 1)
	go graceShutdown()
	return nil
}

// Notify handler for cmds from admin
func (a *Admin) Notify(command string) (string, error) {
	cmd := strings.Split(command, " ")
	// report command to notify
	go ReportNotifyInfo(NOTIFY_NORMAL, "AdminServant::notify:"+command)
	switch cmd[0] {
	case "tars.viewversion":
		return GetServerConfig().Version, nil
	case "tars.setloglevel":
		//if len(cmd) >= 2 {
		//	appCache.LogLevel = cmd[1]
		//	switch cmd[1] {
		//	case "INFO":
		//		logger.SetLevel(logger.INFO)
		//	case "WARN":
		//		logger.SetLevel(logger.WARN)
		//	case "ERROR":
		//		logger.SetLevel(logger.ERROR)
		//	case "DEBUG":
		//		logger.SetLevel(logger.DEBUG)
		//	case "NONE":
		//		logger.SetLevel(logger.OFF)
		//	default:
		//		return fmt.Sprintf("%s failed: unknown log level [%s]!", cmd[0], cmd[1]), nil
		//	}
		//	return fmt.Sprintf("%s succ", command), nil
		//}
		//return fmt.Sprintf("%s failed: missing loglevel!", command), nil
		return setLogLevelCMD(cmd[1:])
	case "tars.dumpstack":
		debug.DumpStack(true, "stackinfo", "tars.dumpstack:")
		return fmt.Sprintf("%s succ", command), nil
	case "tars.loadconfig":
		cfg := GetServerConfig()
		remoteConf := NewRConf(cfg.App, cfg.Server, cfg.BasePath)
		_, err := remoteConf.GetConfig(cmd[1])
		if err != nil {
			return fmt.Sprintf("Getconfig Error!: %s", cmd[1]), err
		}
		return fmt.Sprintf("Getconfig Success!: %s", cmd[1]), nil
	case "tars.connection":
		return fmt.Sprintf("%s not support now!", command), nil
	case "tars.gracerestart":
		graceRestart()
		return "restart gracefully!", nil
	case "tars.pprof":
		port := ":8080"
		timeout := time.Second * 600
		if len(cmd) > 1 {
			port = ":" + cmd[1]
		}
		if len(cmd) > 2 {
			t, _ := strconv.ParseInt(cmd[2], 10, 64)
			if 0 < t && t < 3600 {
				timeout = time.Second * time.Duration(t)
			}
		}
		cfg := GetServerConfig()
		addr := cfg.LocalIP + port
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
			mux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
			mux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
			mux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
			mux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
			s := &http.Server{Addr: addr, Handler: mux}
			zaplog.Info("start serve pprof ", zap.String("Addr", addr))
			go s.ListenAndServe()
			time.Sleep(timeout)
			s.Shutdown(context.Background())
			zaplog.Info("stop serve pprof ", zap.String("Addr", addr))
		}()
		return "see http://" + addr + "/debug/pprof/", nil
	default:
		if fn, ok := adminMethods[cmd[0]]; ok {
			return fn(command)
		}
		return fmt.Sprintf("%s not support now!", command), nil
	}
}

// RegisterAdmin register admin functions
func RegisterAdmin(name string, fn adminFn) {
	adminMethods[name] = fn
}

func setLogLevelCMD(params []string) (string, error) {
	if len(params) >= 1 {
		if err := zaplog.SetLogLevel(strings.ToLower(params[0])); err != nil {
			return "SetLogLevel failed", err
		}
		return "set level to " + params[0], nil
	} else {
		return "no param", errors.New("no param")
	}
}
