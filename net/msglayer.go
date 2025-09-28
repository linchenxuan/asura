package net

type MsgLayerType uint8

const (
	MsgLayerType_None MsgLayerType = iota
	MsgLayerType_Stateless
	MsgLayerType_Stateful
	MsgLayerType_Max
)

// MsgLayer the absolute interface for providers.
type MsgLayer interface {
	MsgLayerReceiver

	Init() error

	// Shutdown called before MsgLayer system shutdown.
	Shutdown()
}
