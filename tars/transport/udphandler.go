package transport

import (
	"context"
	"github.com/MacgradyHuang/TarsGo/tars/util/zaplog"
	"go.uber.org/zap"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/MacgradyHuang/TarsGo/tars/protocol/res/basef"
	"github.com/MacgradyHuang/TarsGo/tars/util/current"
	"github.com/MacgradyHuang/TarsGo/tars/util/grace"
)

type udpHandler struct {
	conf *TarsServerConf
	ts   *TarsServer

	conn      *net.UDPConn
	numInvoke int32
}

func (h *udpHandler) Listen() (err error) {
	cfg := h.conf
	h.conn, err = grace.CreateUDPConn(cfg.Address)
	if err != nil {
		return err
	}
	zaplog.Info("UDP listen", zap.Any("LocalAddr", h.conn.LocalAddr()))
	return nil
}

func (h *udpHandler) Handle() error {
	atomic.AddInt32(&h.ts.numConn, 1)
	///wait invoke done
	defer func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for atomic.LoadInt32(&h.ts.numInvoke) > 0 {
			select {
			case <-tick.C:
			}
		}
		atomic.AddInt32(&h.ts.numConn, -1)
	}()
	buffer := make([]byte, 65535)
	for {
		if atomic.LoadInt32(&h.ts.isClosed) == 1 {
			return nil
		}
		n, udpAddr, err := h.conn.ReadFromUDP(buffer)
		if err != nil {
			if atomic.LoadInt32(&h.ts.isClosed) == 1 {
				return nil
			}
			if isNoDataError(err) {
				continue
			} else {
				zaplog.Error("Close connection", zap.String("Address", h.conf.Address), zap.Error(err))
				return err // TODO: check if necessary
			}
		}
		pkg := make([]byte, n)
		copy(pkg, buffer[0:n])
		go func() {
			ctx := current.ContextWithTarsCurrent(context.Background())
			current.SetClientIPWithContext(ctx, udpAddr.IP.String())
			current.SetClientPortWithContext(ctx, strconv.Itoa(udpAddr.Port))
			current.SetRecvPkgTsFromContext(ctx, time.Now().UnixNano()/1e6)

			atomic.AddInt32(&h.ts.numInvoke, 1)
			rsp := h.ts.invoke(ctx, pkg) // no need to check package

			cPacketType, ok := current.GetPacketTypeFromContext(ctx)
			if !ok {
				zaplog.Error("Failed to GetPacketTypeFromContext")
			}

			if cPacketType == basef.TARSONEWAY {
				atomic.AddInt32(&h.ts.numInvoke, -1)
				return
			}

			if _, err := h.conn.WriteToUDP(rsp, udpAddr); err != nil {
				zaplog.Error("send pkg failed", zap.Any("UdpAddr", udpAddr), zap.Error(err))
			}
			atomic.AddInt32(&h.ts.numInvoke, -1)
		}()
	}
	return nil
}

func (h *udpHandler) OnShutdown() {
}

func (h *udpHandler) CloseIdles(n int64) bool {
	if h.ts.numInvoke == 0 {
		h.conn.Close()
		return true
	}
	return false
}
