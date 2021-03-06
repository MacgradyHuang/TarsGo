package tars

import (
	"context"
	"errors"
	"fmt"
	"github.com/MacgradyHuang/TarsGo/tars/util/zaplog"
	"go.uber.org/zap"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MacgradyHuang/TarsGo/tars/model"
	"github.com/MacgradyHuang/TarsGo/tars/protocol"
	"github.com/MacgradyHuang/TarsGo/tars/protocol/res/basef"
	"github.com/MacgradyHuang/TarsGo/tars/protocol/res/requestf"
	"github.com/MacgradyHuang/TarsGo/tars/util/current"
	"github.com/MacgradyHuang/TarsGo/tars/util/endpoint"
	"github.com/MacgradyHuang/TarsGo/tars/util/rtimer"
	"github.com/MacgradyHuang/TarsGo/tars/util/tools"
)

var (
	maxInt32 int32 = 1<<31 - 1
	msgID    int32
)

const (
	STAT_SUCCESS = iota
	STAT_FAILED
)

// ServantProxy tars servant proxy instance
type ServantProxy struct {
	name     string
	comm     *Communicator
	manager  EndpointManager
	timeout  int
	version  int16
	proto    model.Protocol
	queueLen int32
}

func newServantProxy(comm *Communicator, objName string) *ServantProxy {
	s := &ServantProxy{}
	pos := strings.Index(objName, "@")
	if pos > 0 {
		s.name = objName[0:pos]
	} else {
		s.name = objName
	}
	pos = strings.Index(s.name, "://")
	if pos > 0 {
		s.name = s.name[pos+3:]
	}

	// init manager
	s.manager = GetManager(comm, objName)
	s.comm = comm
	s.proto = &protocol.TarsProtocol{}
	s.timeout = s.comm.Client.AsyncInvokeTimeout
	s.version = basef.TARSVERSION
	return s
}

// TarsSetTimeout sets the timeout for client calling the server , which is in ms.
func (s *ServantProxy) TarsSetTimeout(t int) {
	s.timeout = t
}

// TarsSetVersion set tars version
func (s *ServantProxy) TarsSetVersion(iVersion int16) {
	s.version = iVersion
}

// TarsSetProtocol tars set model protocol
func (s *ServantProxy) TarsSetProtocol(proto model.Protocol) {
	s.proto = proto
}

// Tars_invoke is use for client inoking server.
func (s *ServantProxy) Tars_invoke(ctx context.Context, ctype byte,
	sFuncName string,
	buf []byte,
	status map[string]string,
	reqContext map[string]string,
	resp *requestf.ResponsePacket) error {
	defer CheckPanic()
	//TODO 重置sid，防止溢出
	atomic.CompareAndSwapInt32(&msgID, maxInt32, 1)

	// 将ctx中的dyeinglog信息传入到request中
	var msgType int32
	dyeingKey, ok := current.GetDyeingKey(ctx)
	if ok {
		zaplog.Debug("dyeing debug: find dyeing key:", zap.String("DyeingKey", dyeingKey))
		if status == nil {
			status = make(map[string]string)
		}
		status[current.STATUS_DYED_KEY] = dyeingKey
		msgType = basef.TARSMESSAGETYPEDYED
	}

	req := requestf.RequestPacket{
		IVersion:     s.version,
		CPacketType:  int8(ctype),
		IRequestId:   atomic.AddInt32(&msgID, 1),
		SServantName: s.name,
		SFuncName:    sFuncName,
		SBuffer:      tools.ByteToInt8(buf),
		ITimeout:     s.comm.Client.ReqDefaultTimeout,
		Context:      reqContext,
		Status:       status,
		IMessageType: msgType,
	}
	msg := &Message{Req: &req, Ser: s, Resp: resp}
	msg.Init()
	timeout := time.Duration(s.timeout) * time.Millisecond
	ok, hashType, hashCode, isHash := current.GetClientHash(ctx)
	if ok {
		msg.isHash = isHash
		msg.hashType = HashType(hashType)
		msg.hashCode = hashCode
	}
	ok, to, isTimeout := current.GetClientTimeout(ctx)
	if ok && isTimeout {
		timeout = time.Duration(to) * time.Millisecond
	}
	var err error
	s.manager.preInvoke()
	if allFilters.cf != nil {
		err = allFilters.cf(ctx, msg, s.doInvoke, timeout)
	} else {
		// execute pre client filters
		for i, v := range allFilters.preCfs {
			err = v(ctx, msg, s.doInvoke, timeout)
			if err != nil {
				zaplog.Error("Pre filter error", zap.Int("Index", i), zap.Error(err))
			}
		}
		// execute rpc
		err = s.doInvoke(ctx, msg, timeout)
		// execute post client filters
		for i, v := range allFilters.postCfs {
			err = v(ctx, msg, s.doInvoke, timeout)
			if err != nil {
				zaplog.Error("Post filter error", zap.Int("Index", i), zap.Error(err))
			}
		}
	}
	s.manager.postInvoke()

	if err != nil {
		msg.End()
		zaplog.Error("Invoke error", zap.String("Name", s.name), zap.String("FuncName", sFuncName), zap.Int64("Cost", msg.Cost()), zap.Error(err))
		if msg.Resp == nil {
			ReportStat(msg, STAT_SUCCESS, STAT_SUCCESS, STAT_FAILED)
		} else if msg.Status == basef.TARSINVOKETIMEOUT {
			ReportStat(msg, STAT_SUCCESS, STAT_FAILED, STAT_SUCCESS)
		} else {
			ReportStat(msg, STAT_SUCCESS, STAT_SUCCESS, STAT_FAILED)
		}
		return err
	}
	msg.End()
	*resp = *msg.Resp
	ReportStat(msg, STAT_FAILED, STAT_SUCCESS, STAT_SUCCESS)
	return err
}

func (s *ServantProxy) doInvoke(ctx context.Context, msg *Message, timeout time.Duration) error {
	adp, needCheck := s.manager.SelectAdapterProxy(msg)
	if adp == nil {
		return errors.New("no adapter Proxy selected:" + msg.Req.SServantName)
	}
	if s.queueLen > ObjQueueMax {
		return errors.New("invoke queue is full:" + msg.Req.SServantName)
	}
	ep := adp.GetPoint()
	current.SetServerIPWithContext(ctx, ep.Host)
	current.SetServerPortWithContext(ctx, fmt.Sprintf("%v", ep.Port))
	msg.Adp = adp
	adp.obj = s
	atomic.AddInt32(&s.queueLen, 1)
	readCh := make(chan *requestf.ResponsePacket)
	adp.resp.Store(msg.Req.IRequestId, readCh)
	defer func() {
		CheckPanic()
		atomic.AddInt32(&s.queueLen, -1)
		adp.resp.Delete(msg.Req.IRequestId)
	}()
	if err := adp.Send(msg.Req); err != nil {
		adp.failAdd()
		return err
	}
	if msg.Req.CPacketType == basef.TARSONEWAY {
		adp.succssAdd()
		return nil
	}
	select {
	case <-rtimer.After(timeout):
		msg.Status = basef.TARSINVOKETIMEOUT
		adp.failAdd()
		msg.End()
		return fmt.Errorf("request timeout, begin time:%d, cost:%d, obj:%s, func:%s, addr:(%s:%d), reqid:%d",
			msg.BeginTime, msg.Cost(), msg.Req.SServantName, msg.Req.SFuncName, adp.point.Host, adp.point.Port, msg.Req.IRequestId)
	case msg.Resp = <-readCh:
		if needCheck {
			go func() {
				adp.reset()
				ep := endpoint.Tars2endpoint(*msg.Adp.point)
				s.manager.addAliveEp(ep)
			}()
		}
		adp.succssAdd()
		if msg.Resp != nil {
			if msg.Status != basef.TARSSERVERSUCCESS || msg.Resp.IRet != 0 {
				if msg.Resp.SResultDesc == "" {
					return fmt.Errorf("basef error code %d", msg.Resp.IRet)
				}
				return errors.New(msg.Resp.SResultDesc)
			}
		} else {
			zaplog.Debug("recv nil Resp, close of the readCh?")
		}
		zaplog.Debug("recv msg success", zap.Int32("IRequestId", msg.Req.IRequestId))
	}
	return nil
}
