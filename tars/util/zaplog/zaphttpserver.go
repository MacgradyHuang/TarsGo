package zaplog

import (
	"go.uber.org/zap"
	"net"
	"net/http"
)

var zapLoggerHttpServer string

func runZapLoggerHttpServer(config *zapLoggerConf, level zap.AtomicLevel) {
	mux := http.NewServeMux()
	mux.Handle(config.logApiPath, level)
	listener, err := net.Listen("tcp", config.listenAddr)
	if err != nil {
		Fatal("runZapLoggerHttpServer err", zap.String("ListenAddr", config.listenAddr), zap.Error(err))
	} else {
		zapLoggerHttpServer = "http://" + listener.Addr().String() + config.logApiPath
		Info("make zapLoggerHttpServer success", zap.String("ZapLoggerHttpServer", zapLoggerHttpServer))
	}
	go func() {
		if err = http.Serve(listener, mux); err != nil {
			Fatal("runZapLoggerHttpServer err", zap.String("ListenAddr", config.listenAddr), zap.Error(err))
		}
	}()
}
