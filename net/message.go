// Package net provides core networking capabilities for the Asura distributed game server framework.
// This file specifically defines message types, protocols, and creation utilities.
package net

import (
	"errors"

	"github.com/lcx/asura/utils"
	"google.golang.org/protobuf/proto"
)

// MsgReqType defines the type of message requests in the Asura framework.
type MsgReqType int

const (
	// MRTNone represents an invalid or uninitialized message type.
	MRTNone MsgReqType = iota
	// MRTReq indicates a request message that expects a response.
	MRTReq
	// MRTRes indicates a response message to a previous request.
	MRTRes
	// MRTNtf indicates a notification message that does not require a response.
	MRTNtf
)

// MsgProtoInfo contains metadata and handling information for a specific protocol message.
// This structure is used by the framework to manage message routing, processing, and response generation.
type MsgProtoInfo struct {
	New          func() proto.Message // Factory function to create new instances of the message
	MsgID        string               // Unique identifier for the message type
	ResMsgID     string               // Message ID of the corresponding response message (if applicable)
	MsgReqType   MsgReqType           // Type of the message (request, response, notification)
	IsCS         bool                 // Indicates if this is a client-server message
	MsgHandle    any                  // Handler function for processing the message
	MsgLayerType MsgLayerType         // Layer type that should handle this message
	HasRecvSvr   bool                 // Indicates if the message has a receiving server
}

// IsNtf checks if the protocol message is a notification type.
func (pi *MsgProtoInfo) IsNtf() bool {
	if pi != nil {
		return pi.MsgReqType == MRTNtf
	}
	return false
}

// IsSSReq checks if the protocol message is a server-to-server request.
func (pi *MsgProtoInfo) IsSSReq() bool {
	if pi != nil {
		return pi.MsgReqType == MRTReq && !pi.IsCS
	}
	return false
}

// IsSSRes checks if the protocol message is a server-to-server response.
func (pi *MsgProtoInfo) IsSSRes() bool {
	if pi != nil {
		return pi.MsgReqType == MRTRes && !pi.IsCS
	}
	return false
}

// IsReq checks if the protocol message is any type of request (CS or SS).
func (pi *MsgProtoInfo) IsReq() bool {
	if pi != nil {
		return pi.MsgReqType == MRTReq
	}
	return false
}

// IsRes checks if the protocol message is any type of response (CS or SS).
func (pi *MsgProtoInfo) IsRes() bool {
	if pi != nil {
		return pi.MsgReqType == MRTRes
	}
	return false
}

// GetResMsgID retrieves the response message ID associated with this request message.
func (pi *MsgProtoInfo) GetResMsgID() string {
	if pi != nil {
		return pi.ResMsgID
	}
	return ""
}

// GetMsgID retrieves the unique identifier for this message type.
func (pi *MsgProtoInfo) GetMsgID() string {
	if pi != nil {
		return pi.MsgID
	}
	return ""
}

// GetMsgHandle retrieves the handler function registered for processing this message type.
func (pi *MsgProtoInfo) GetMsgHandle() any {
	if pi != nil {
		return pi.MsgHandle
	}
	return nil
}

// MsgCreator defines the interface for creating protocol buffer messages based on message IDs.
// This interface enables dynamic message instantiation in distributed game server scenarios,
// supporting runtime message type resolution without tight coupling to specific implementations.
type MsgCreator interface {
	// CreateMsg creates a new protobuf message instance based on the provided message ID.
	// The message ID typically corresponds to game-specific message types (e.g., "PlayerMove", "ChatMessage").
	CreateMsg(msgID string) (proto.Message, error)

	// ContainsMsg checks if the creator can handle the specified message ID.
	// Used for validation before attempting message creation to prevent runtime errors.
	ContainsMsg(msgID string) bool
}

// NewPkgHead creates a new package header with the specified message ID.
// Package headers contain metadata about messages being transmitted through the framework.
func NewPkgHead(msgID string) *PkgHead {
	return &PkgHead{
		MsgID: msgID,
	}
}

// AsuraMsg is the base interface that all protocol messages must implement to be transferred
// within the Asura framework. It extends proto.Message to support Protocol Buffer serialization.
type AsuraMsg interface {
	proto.Message
	// MsgID returns the unique identifier of the protocol message used for routing.
	MsgID() string
	// IsRequest checks if this message represents a request that expects a response.
	IsRequest() bool
	// IsResponse checks if this message represents a response to a previous request.
	IsResponse() bool
}

// SCNtfMsg defines the interface for server-to-client notification messages.
// These messages are pushed from servers to clients without expecting a direct response.
type SCNtfMsg interface {
	AsuraMsg
	// SCNtfMsgID returns the same value as AsuraMsg.MsgID for consistency.
	SCNtfMsgID() string
	// NtfClient sends the notification package to a specific client entity and actor.
	NtfClient(sender NtfPkgSender, dstEntity uint32, dstActorID uint64, opts ...TransPkgOption)
	// DeferNtfClient schedules a delayed notification to be sent to a client entity and actor.
	DeferNtfClient(sender DeferPkgSender, dstEntity uint32, dstActorID uint64, opts ...TransPkgOption)
}

// SSMsg defines the interface for server-to-server messages in the Asura framework.
// These messages facilitate communication between different server components.
type SSMsg interface {
	AsuraMsg
	// IsSS confirms that this is a server-to-server message.
	IsSS() bool
}

// RPCReqMsg defines the interface for remote procedure call request messages.
// These messages initiate a function call on a remote server component and expect a response.
type RPCReqMsg interface {
	AsuraMsg

	// ResMsgID returns the message ID of the corresponding response message.
	ResMsgID() string
	// CreateResMsg creates a new instance of the response message type associated with this request.
	CreateResMsg() proto.Message
}

// NewResPkg creates a response package based on an incoming request package.
// This function automatically sets up routing information to send the response back to the request originator.
// Parameters:
// - reqPkg: The incoming request package
// - resMsgID: Message ID for the response
// - retCode: Return code indicating success or error condition
// - resBody: The actual response message content
// - opts: Optional transport package configuration options
func NewResPkg(reqPkg *TransRecvPkg, resMsgID string, retCode int32,
	resBody proto.Message, opts ...TransPkgOption) (*TransSendPkg, error) {
	if reqPkg == nil {
		return nil, errors.New("reqPkg is nil")
	}

	if resMsgID == "" {
		return nil, errors.New("no res msgid")
	}
	resPkg := &TransSendPkg{
		RouteHdr: &RouteHead{
			SrcEntityID: utils.GetEntityID(),
			RouteType: &RouteHead_P2P{
				P2P: &P2PRoute{
					DstEntityID: reqPkg.RouteHdr.GetSrcEntityID(),
				},
			},
			MsgID: resMsgID,
		},
		PkgHdr: NewPkgHead(resMsgID),
		Body:   resBody,
	}

	resPkg.PkgHdr.RetCode = retCode
	resPkg.PkgHdr.DstActorID = reqPkg.PkgHdr.GetSrcActorID()
	resPkg.PkgHdr.SrcActorID = reqPkg.PkgHdr.GetDstActorID()

	for _, opt := range opts {
		opt(resPkg)
	}
	return resPkg, nil
}

// NewSCNtfPkg builds a server-to-client notification package.
// This function sets up the necessary routing information to deliver notifications to specific clients.
// Parameters:
// - m: The notification message implementing SCNtfMsg
// - dstEntity: Target entity ID
// - dstActorID: Target actor ID
// - opts: Optional transport package configuration options
func NewSCNtfPkg(m SCNtfMsg, dstEntity uint32, dstActorID uint64, opts ...TransPkgOption) TransSendPkg {
	h := TransSendPkg{
		PkgHdr: NewPkgHead(m.MsgID()),
		Body:   m,
	}
	for _, opt := range opts {
		opt(&h)
	}

	// Set destination actor ID
	h.SetDstActorID(dstActorID)

	// Set up cross-entity routing if destination is different from current entity
	if dstEntity != utils.GetEntityID() {
		h.SetRouteHdr(&RouteHead{
			SrcEntityID: utils.GetEntityID(),
			RouteType: &RouteHead_P2P{
				P2P: &P2PRoute{
					DstEntityID: dstEntity,
				},
			},
		})
	}

	return h
}

// NewPostLocalPkg builds a package for local message delivery within the same server.
// This is optimized for internal communication that doesn't need to cross server boundaries.
// Parameters:
// - m: The message to be sent locally
// - opts: Optional transport package configuration options
func NewPostLocalPkg(m AsuraMsg, opts ...TransPkgOption) TransSendPkg {
	h := TransSendPkg{
		RouteHdr: &RouteHead{
			SrcEntityID: 0, // Local messages don't need source entity ID
			RouteType: &RouteHead_P2P{
				P2P: &P2PRoute{
					DstEntityID: utils.GetEntityID(), // Target is current server
				},
			},
			MsgID: m.MsgID(),
		},
		PkgHdr: NewPkgHead(m.MsgID()),
		Body:   m,
	}
	for _, opt := range opts {
		opt(&h)
	}
	return h
}

// NewP2PPkg builds a point-to-point server-to-server notification package.
// This is used for direct communication between two server instances.
// Parameters:
// - m: The server-to-server message implementing SSMsg
// - dstID: Target server entity ID
// - opts: Optional transport package configuration options
func NewP2PPkg(m SSMsg, dstID uint32, opts ...TransPkgOption) TransSendPkg {
	h := TransSendPkg{
		RouteHdr: &RouteHead{
			SrcEntityID: utils.GetEntityID(),
			RouteType: &RouteHead_P2P{
				P2P: &P2PRoute{
					DstEntityID: dstID,
				},
			},
			MsgID: m.MsgID(),
		},
		PkgHdr: NewPkgHead(m.MsgID()),
		Body:   m,
	}

	for _, opt := range opts {
		opt(&h)
	}
	return h
}

// NewP2PPkgActor builds a point-to-point server-to-server notification package
// targeted at a specific actor within the destination server.
// Parameters:
// - m: The server-to-server message implementing SSMsg
// - dstEntityID: Target server entity ID
// - dstActorID: Target actor ID within the destination server
// - opts: Optional transport package configuration options
func NewP2PPkgActor(m SSMsg, dstEntityID uint32, dstActorID uint64, opts ...TransPkgOption) TransSendPkg {
	pkg := NewP2PPkg(m, dstEntityID, opts...)
	pkg.PkgHdr.DstActorID = dstActorID
	return pkg
}
