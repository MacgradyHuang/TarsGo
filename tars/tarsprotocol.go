package tars

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/MacgradyHuang/TarsGo/tars/util/zaplog"
	"go.uber.org/zap"
	"time"

	"github.com/MacgradyHuang/TarsGo/tars/protocol"
	"github.com/MacgradyHuang/TarsGo/tars/protocol/codec"
	"github.com/MacgradyHuang/TarsGo/tars/protocol/res/basef"
	"github.com/MacgradyHuang/TarsGo/tars/protocol/res/requestf"
	"github.com/MacgradyHuang/TarsGo/tars/util/current"
)

type dispatch interface {
	Dispatch(context.Context, interface{}, *requestf.RequestPacket, *requestf.ResponsePacket, bool) error
}

// TarsProtocol is struct for dispatch with tars protocol.
type TarsProtocol struct {
	dispatcher       dispatch
	serverImp        interface{}
	withContext      bool
}

// NewTarsProtocol return a TarsProtocol with dipatcher and implement interface.
// withContext explain using context or not.
func NewTarsProtocol(dispatcher dispatch, imp interface{}, withContext bool) *TarsProtocol {
	s := &TarsProtocol{dispatcher: dispatcher, serverImp: imp, withContext: withContext}
	return s
}

// Invoke puts the request as []byte and call the dispather, and then return the response as []byte.
func (s *TarsProtocol) Invoke(ctx context.Context, req []byte) (rsp []byte) {
	defer CheckPanic()
	reqPackage := requestf.RequestPacket{}
	rspPackage := requestf.ResponsePacket{}
	is := codec.NewReader(req[4:])
	reqPackage.ReadFrom(is)

	if reqPackage.HasMessageType(basef.TARSMESSAGETYPEDYED) {
		if dyeingKey, ok := reqPackage.Status[current.STATUS_DYED_KEY]; ok {
			if ok := current.SetDyeingKey(ctx, dyeingKey); !ok {
				zaplog.Error("dyeing-debug: set dyeing key in current status error, dyeing key:", zap.String("DyeingKey", dyeingKey))
			}
		}
	}

	if reqPackage.CPacketType == basef.TARSONEWAY {
		defer func() func() {
			beginTime := time.Now().UnixNano() / 1e6
			return func() {
				endTime := time.Now().UnixNano() / 1e6
				ReportStatFromServer(reqPackage.SFuncName, "one_way_client", rspPackage.IRet, endTime-beginTime)
			}
		}()()
	}
	var err error
	if s.withContext {
		ok := current.SetRequestStatus(ctx, reqPackage.Status)
		if !ok {
			zaplog.Error("Set reqeust status in context fail!")
		}
		ok = current.SetRequestContext(ctx, reqPackage.Context)
		if !ok {
			zaplog.Error("Set request context in context fail!")
		}
	}
	if allFilters.sf != nil {
		err = allFilters.sf(ctx, s.dispatcher.Dispatch, s.serverImp, &reqPackage, &rspPackage, s.withContext)
	} else {
		// execute pre server filters
		for i, v := range allFilters.preSfs {
			err = v(ctx, s.dispatcher.Dispatch, s.serverImp, &reqPackage, &rspPackage, s.withContext)
			if err != nil {
				zaplog.Error("Pre filter error", zap.Int("Index", i), zap.Error(err))
			}
		}
		err = s.dispatcher.Dispatch(ctx, s.serverImp, &reqPackage, &rspPackage, s.withContext)
		// execute post server filters
		for i, v := range allFilters.postSfs {
			err = v(ctx, s.dispatcher.Dispatch, s.serverImp, &reqPackage, &rspPackage, s.withContext)
			if err != nil {
				zaplog.Error("Post filter error", zap.Int("Index", i), zap.Error(err))
			}
		}
	}
	if err != nil {
		zaplog.Error("found err", zap.Int32("IRequestId", reqPackage.IRequestId), zap.Error(err))
		rspPackage.IVersion = basef.TARSVERSION
		rspPackage.CPacketType = basef.TARSNORMAL
		rspPackage.IRequestId = reqPackage.IRequestId
		rspPackage.IRet = 1
		rspPackage.SResultDesc = err.Error()
	}

	//return ctype
	rspPackage.CPacketType = reqPackage.CPacketType
	ok := current.SetPacketTypeFromContext(ctx, rspPackage.CPacketType)
	if !ok {
		zaplog.Error("SetPacketType in context fail!")
	}

	return s.rsp2Byte(&rspPackage)
}

func (s *TarsProtocol) rsp2Byte(rsp *requestf.ResponsePacket) []byte {
	os := codec.NewBuffer()
	rsp.WriteTo(os)
	bs := os.ToBytes()
	sbuf := bytes.NewBuffer(nil)
	sbuf.Write(make([]byte, 4))
	sbuf.Write(bs)
	len := sbuf.Len()
	binary.BigEndian.PutUint32(sbuf.Bytes(), uint32(len))
	return sbuf.Bytes()
}

// ParsePackage parse the []byte according to the tars protocol.
// returns header length and package integrity condition (PACKAGE_LESS | PACKAGE_FULL | PACKAGE_ERROR)
func (s *TarsProtocol) ParsePackage(buff []byte) (int, int) {
	return protocol.TarsRequest(buff)
}

// InvokeTimeout indicates how to deal with timeout.
func (s *TarsProtocol) InvokeTimeout(pkg []byte) []byte {
	rspPackage := requestf.ResponsePacket{}
	rspPackage.IRet = 1
	rspPackage.SResultDesc = "server invoke timeout"
	return s.rsp2Byte(&rspPackage)
}

// GetCloseMsg return a package to close connection
func (s *TarsProtocol) GetCloseMsg() []byte {
	rspPackage := requestf.ResponsePacket{}
	rspPackage.IRequestId = 0
	rspPackage.SResultDesc = reconnectMsg
	return s.rsp2Byte(&rspPackage)
}
