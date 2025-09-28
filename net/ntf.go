package net

import (
	"errors"
	"fmt"
)

// NtfSender .
type NtfSender struct {
	msgManager *MessageManager
}

func NewNtfSender(msgManager *MessageManager) *NtfSender {
	return &NtfSender{
		msgManager: msgManager,
	}
}

// NtfClientImpl send pkg to client.
func (s *NtfSender) NtfClient(pkg TransSendPkg, aid uint64, cs CSTransport) error {
	if pkg.PkgHdr == nil {
		return errors.New("ntf pkg header is nil")
	}
	if cs == nil {
		return fmt.Errorf("MsgID:%s ntf CSTransport nil", pkg.PkgHdr.GetMsgID())
	}

	info, ok := s.msgManager.GetProtoInfo(pkg.PkgHdr.GetMsgID())
	if !ok || info == nil {
		return fmt.Errorf("MsgID:%s ntf no proto info", pkg.PkgHdr.GetMsgID())
	}

	if !s.msgManager.IsCSNtfMsg(pkg.PkgHdr.GetMsgID()) {
		return fmt.Errorf("MsgID:%s ntf is not csntf msg", pkg.PkgHdr.GetMsgID())
	}
	if aid > 0 {
		pkg.PkgHdr.DstActorID = aid
	}

	return cs.SendToClient(pkg)
}

// NtfServerImpl send pkg to another server.
func (s *NtfSender) NtfServer(pkg TransSendPkg, ss SSTransport) error {
	if pkg.PkgHdr == nil {
		return errors.New("ntf pkg header is nil")
	}

	info, ok := s.msgManager.GetProtoInfo(pkg.PkgHdr.GetMsgID())
	if !ok || info == nil {
		return fmt.Errorf("MsgID:%s ntf no proto info", pkg.PkgHdr.GetMsgID())
	}

	if !s.msgManager.IsNtfMsg(pkg.PkgHdr.GetMsgID()) {
		return fmt.Errorf("MsgID:%s ntf is not ntf", pkg.PkgHdr.GetMsgID())
	}

	return ss.SendToServer(pkg)
}
