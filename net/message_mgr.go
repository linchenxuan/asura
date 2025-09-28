// Package net provides core networking capabilities for the Asura distributed game server framework.
// This file implements the message manager responsible for message protocol registration and management.
package net

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

// MessageManager is the central manager for message protocol information in the Asura framework.
// It maintains a registry of message types, their properties, and handling functions.
type MessageManager struct {
	// PropInfoMap stores protocol information for each message ID
	PropInfoMap map[string]*MsgProtoInfo
}

// NewMessageManager creates a new instance of MessageManager with initialized storage.
func NewMessageManager() *MessageManager {
	return &MessageManager{
		PropInfoMap: make(map[string]*MsgProtoInfo),
	}
}

// RegisterMsgInfo registers complete message protocol information in the manager.
// If a message with the same ID was previously registered, it preserves the existing handler
// and message layer type while updating other information.
// Parameters:
// - pi: The message protocol information to register
func (m *MessageManager) RegisterMsgInfo(pi *MsgProtoInfo) {
	// Validate input parameters
	if pi == nil || len(pi.MsgID) == 0 || pi.New == nil {
		// printer.Error().Any("pi", pi).Msg("reg invalid")
		return
	}

	// Preserve existing handler and layer type if already registered
	if p, ok := m.PropInfoMap[pi.MsgID]; ok {
		pi.MsgHandle = p.MsgHandle
		pi.MsgLayerType = p.MsgLayerType
	}
	// Store the protocol information
	m.PropInfoMap[pi.MsgID] = pi
}

// RegisterMsgHandle registers a message handler function and message layer type for a specific message ID.
// If the message ID is already registered, it updates the handler and layer type.
// If not registered, it creates a minimal entry with just the handler and layer type.
// Parameters:
// - msgid: The message ID to register the handler for
// - handle: The handler function to process messages of this type
// - msgLayerType: The message layer that should handle this message
func (m *MessageManager) RegisterMsgHandle(msgid string, handle any, msgLayerType MsgLayerType) {
	// Validate message ID
	if len(msgid) == 0 {
		return
	}

	// Update existing registration if found
	if p, ok := m.PropInfoMap[msgid]; ok {
		p.MsgHandle = handle
		p.MsgLayerType = msgLayerType
		return
	}
	// Create new minimal registration if not found
	m.PropInfoMap[msgid] = &MsgProtoInfo{
		MsgHandle:    handle,
		MsgLayerType: msgLayerType,
	}
}

// GetProtoInfo retrieves the protocol information for a given message ID.
// Parameters:
// - msgID: The message ID to look up
// Returns:
// - The protocol information if found, along with a boolean indicating success
func (m *MessageManager) GetProtoInfo(msgID string) (*MsgProtoInfo, bool) {
	protoInfo, ok := m.PropInfoMap[msgID]
	return protoInfo, ok
}

// CreateMsg creates a new message instance based on the provided message ID.
// This uses the factory function registered for that message type.
// Parameters:
// - msgID: The message ID to create an instance of
// Returns:
// - A new message instance if the message ID is registered, or an error if not found
func (m *MessageManager) CreateMsg(msgID string) (proto.Message, error) {
	info, ok := m.GetProtoInfo(msgID)
	if !ok {
		return nil, errors.New("msgid not found")
	}
	return info.New(), nil
}

// ContainsMsg checks if a message ID is registered in the manager.
// Parameters:
// - msgID: The message ID to check
// Returns:
// - True if the message ID is registered, false otherwise
func (m *MessageManager) ContainsMsg(msgID string) bool {
	_, ok := m.GetProtoInfo(msgID)
	return ok
}

// IsRequestMsg determines if a message ID represents a request message type.
// Parameters:
// - msgID: The message ID to check
// Returns:
// - True if the message is a request type, false otherwise or if not found
func (m *MessageManager) IsRequestMsg(msgID string) bool {
	info, ok := m.GetProtoInfo(msgID)
	if !ok {
		return false
	}
	return info.MsgReqType == MRTReq
}

// IsNtfMsg determines if a message ID represents a notification message type.
// Parameters:
// - msgID: The message ID to check
// Returns:
// - True if the message is a notification type, false otherwise or if not found
func (m *MessageManager) IsNtfMsg(msgID string) bool {
	info, ok := m.GetProtoInfo(msgID)
	if !ok {
		return false
	}
	return info.MsgReqType == MRTNtf
}

// IsSSRequestMsg determines if a message ID represents a server-to-server request message.
// Parameters:
// - msgID: The message ID to check
// Returns:
// - True if the message is a server-to-server request, false otherwise or if not found
func (m *MessageManager) IsSSRequestMsg(msgID string) bool {
	info, ok := m.GetProtoInfo(msgID)
	if !ok {
		return false
	}
	return info.IsSSReq()
}

// IsSSResMsg determines if a message ID represents a server-to-server response message.
// Parameters:
// - msgID: The message ID to check
// Returns:
// - True if the message is a server-to-server response, false otherwise or if not found
func (m *MessageManager) IsSSResMsg(msgID string) bool {
	info, ok := m.GetProtoInfo(msgID)
	if !ok {
		return false
	}
	return info.IsSSRes()
}

// IsCSNtfMsg determines if a message ID represents a client-server notification message.
// Parameters:
// - msgID: The message ID to check
// Returns:
// - True if the message is a client-server notification, false otherwise or if not found
func (m *MessageManager) IsCSNtfMsg(msgID string) bool {
	info, ok := m.GetProtoInfo(msgID)
	if !ok {
		return false
	}
	return info.MsgReqType == MRTNtf && info.IsCS
}

// GetAllMsgList retrieves all message IDs that satisfy the provided check function.
// This allows for flexible filtering of registered messages based on custom criteria.
// Parameters:
// - checkFunc: A function that evaluates whether a message should be included in the result
// Returns:
// - A list of message IDs that passed the check function
func (m *MessageManager) GetAllMsgList(checkFunc func(protoInfo *MsgProtoInfo) bool) []string {
	// Preallocate slice with estimated capacity
	msgList := make([]string, 0, len(m.PropInfoMap))
	// Iterate through all registered messages and apply filter
	for msgID, protoInfo := range m.PropInfoMap {
		if !checkFunc(protoInfo) {
			continue
		}
		msgList = append(msgList, msgID)
	}
	return msgList
}
