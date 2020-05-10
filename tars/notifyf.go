package tars

import (
	"github.com/MacgradyHuang/TarsGo/tars/protocol/res/notifyf"
	"github.com/MacgradyHuang/TarsGo/tars/util/zaplog"
	"go.uber.org/zap"
)

const (
	NOTIFY_NORMAL = 0
	NOTIFY_WARN   = 1
	NOTIFY_ERROR  = 2
)

// NotifyHelper is the helper struct for the Notify service.
type NotifyHelper struct {
	comm *Communicator
	tn   *notifyf.Notify
	tm   notifyf.ReportInfo
}

// SetNotifyInfo sets the communicator's notify info with communicator, notify name, app name, server name, and container name
func (n *NotifyHelper) SetNotifyInfo(comm *Communicator, notify string, app string, server string, container string) {
	n.comm = comm
	n.tn = new(notifyf.Notify)
	comm.StringToProxy(notify, n.tn)
	//TODO:params
	var set string
	if v, ok := comm.GetProperty("setdivision"); ok {
		set = v
	}
	n.tm = notifyf.ReportInfo{
		0,
		app,
		set,
		container,
		server,
		"",
		"",
		0,
	}
}

// ReportNotifyInfo reports notify information with level and info
func (n *NotifyHelper) ReportNotifyInfo(level int32, info string) {
	n.tm.ELevel = notifyf.NOTIFYLEVEL(level)
	n.tm.SMessage = info
	zaplog.Debug("Notify", zap.Any("Type", n.tm))
	n.tn.ReportNotifyInfo(&n.tm)
}

// ReportNotifyInfo reports notify information with level and info
func ReportNotifyInfo(level int32, info string) {
	ha := new(NotifyHelper)
	comm := NewCommunicator()
	notify := GetServerConfig().Notify
	if notify == "" {
		return
	}
	app := GetServerConfig().App
	server := GetServerConfig().Server
	container := GetServerConfig().Container
	ha.SetNotifyInfo(comm, notify, app, server, container)
	defer func() {
		if err := recover(); err != nil {
			zaplog.Debug("found err", zap.Any("Error", err))
		}
	}()
	ha.ReportNotifyInfo(level, info)
}
