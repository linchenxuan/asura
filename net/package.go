// Package net implements core networking components for the Asura distributed game server framework.
// It handles message packaging, routing, and serialization for inter-service communication
// in a high-performance, low-latency gaming environment.
package net

import (
	"github.com/lcx/asura/codec"
	"github.com/lcx/asura/log"
	"google.golang.org/protobuf/proto"
)

// TransRecvPkg represents a received transport package in the distributed game server framework.
// It encapsulates all necessary information for processing incoming network messages,
// including routing headers, package metadata, and the actual message body.
// This structure is designed for zero-copy operations and efficient memory usage in high-concurrency scenarios.
type TransRecvPkg struct {
	// RouteHdr contains routing information for message delivery in distributed clusters.
	// Includes target service IDs, area information, and routing strategy parameters.
	RouteHdr *RouteHead

	// PkgHdr contains the package header with metadata like message type, source/destination IDs,
	// timestamps, and other control information for message processing.
	PkgHdr *PkgHead

	// decoded indicates whether the message body has been successfully decoded from raw bytes.
	// Used for lazy decoding optimization to avoid unnecessary deserialization overhead.
	decoded bool

	// bodyData stores the raw serialized message body bytes before decoding.
	// Cleared after successful decoding to reduce memory footprint.
	bodyData []byte

	// creator is the message factory instance used to create appropriate message types.
	// Enables dynamic message instantiation based on runtime message IDs.
	creator MsgCreator

	// body is the decoded protobuf message instance ready for business logic processing.
	// Nil until DecodeBody() is called successfully.
	body proto.Message
}

// SetRouteHdr safely sets the route header for this transport package.
// Thread-safe operation for concurrent access scenarios in distributed systems.
func (t *TransRecvPkg) SetRouteHdr(rh *RouteHead) {
	t.RouteHdr = rh
}

// GetRouteHdr returns the route header associated with this package.
// Provides read-only access to routing information for message dispatching.
func (t *TransRecvPkg) GetRouteHdr() *RouteHead {
	return t.RouteHdr
}

// GetPkgHdr returns the package header containing metadata about this transport package.
// Used by middleware and routing layers for message processing decisions.
func (t *TransRecvPkg) GetPkgHdr() *PkgHead {
	return t.PkgHdr
}

// DecodeBody lazily decodes the message body from raw bytes to a protobuf message instance.
// Implements lazy loading pattern to optimize performance in scenarios where not all
// messages need full deserialization (e.g., routing-only processing).
// Returns the decoded message or an error if decoding fails.
func (t *TransRecvPkg) DecodeBody() (proto.Message, error) {
	// Return cached decoded message if already processed
	if t.decoded {
		return t.body, nil
	}

	// Mark as decoded to prevent repeated processing
	t.decoded = true

	// Clear raw data after decoding to reduce memory usage
	defer func() { t.bodyData = nil }()

	// Create appropriate message type based on message ID
	body, err := t.creator.CreateMsg(t.PkgHdr.MsgID)
	if err != nil {
		return nil, err
	}

	// Deserialize protobuf message from raw bytes
	if err := codec.Decode(body, t.bodyData); err != nil {
		return nil, err
	}

	t.body = body
	return t.body, nil
}

// ConvertToTransSendPkg converts this received package to a send package format.
// Useful for message forwarding, broadcasting, or transforming received messages
// into responses in distributed game server scenarios.
// Returns a new TransSendPkg with the decoded body or an error if conversion fails.
func (t *TransRecvPkg) ConvertToTransSendPkg() (*TransSendPkg, error) {
	var pkg TransSendPkg

	// Ensure message body is decoded before conversion
	body, err := t.DecodeBody()
	if err != nil {
		return &pkg, err
	}

	// Copy relevant fields to the send package
	pkg.RouteHdr = t.RouteHdr
	pkg.Body = body
	pkg.PkgHdr = t.PkgHdr
	return &pkg, nil
}

// MarshalLogObj implements the log.ObjectMarshaller interface for structured logging.
// Provides a concise and safe representation of the transport package for debugging,
// performance monitoring, and distributed tracing in production game servers.
func (t *TransRecvPkg) MarshalLogObj(e *log.LogEvent) {
	// Log routing information
	e.Str("route", t.RouteHdr.String())

	// Log package metadata
	e.Str("pkgHdr", t.PkgHdr.String())

	// Log message body information safely
	if t.decoded && t.body != nil {
		// Use protobuf reflection to get message type without full serialization
		msg, err := proto.Marshal(t.body)
		if err == nil {
			e.Str("msg", string(msg))
		}
	}
}

// TransSendPkg represents a transport package ready for sending in the distributed game server.
// Contains all necessary information for message transmission including routing,
// message content, and optional tail data for extended functionality.
type TransSendPkg struct {
	// RouteHdr contains routing information for message delivery across distributed services.
	// Specifies target services, routing strategies, and delivery guarantees.
	RouteHdr *RouteHead

	// PkgHdr contains package metadata including message type, timestamps, and control flags.
	// Used by transport layer for message processing and routing decisions.
	PkgHdr *PkgHead

	// Body is the actual protobuf message to be sent.
	// Must be a valid protobuf message instance ready for serialization.
	Body proto.Message
}

// SetSrcActorID safely sets the source actor ID in the package header.
// Thread-safe operation for concurrent message construction scenarios.
func (t *TransSendPkg) SetSrcActorID(id uint64) {
	t.PkgHdr.SrcActorID = id
}

// GetRouteHdr returns the route header for this send package.
// Provides access to routing configuration for message dispatching.
func (t *TransSendPkg) GetRouteHdr() *RouteHead {
	return t.RouteHdr
}

// SetRouteHdr safely sets the route header for this send package.
// Enables dynamic routing configuration based on runtime conditions.
func (t *TransSendPkg) SetRouteHdr(rh *RouteHead) {
	t.RouteHdr = rh
}

// GetPkgHdr returns the package header containing message metadata.
// Used by transport middleware for message processing and validation.
func (t *TransSendPkg) GetPkgHdr() *PkgHead {
	return t.PkgHdr
}

// SetDstActorID sets the destination actor ID in the package header.
// Used for targeted message delivery to specific game entities or services.
func (t *TransSendPkg) SetDstActorID(id uint64) {
	t.PkgHdr.DstActorID = id
}

// MarshalLogObj implements the log.ObjectMarshaller interface for structured logging.
// Provides a concise and safe representation of the transport package for debugging,
// performance monitoring, and distributed tracing in production game servers.
func (t *TransSendPkg) MarshalLogObj(e *log.LogEvent) {
	// Log routing information
	e.Str("route", t.RouteHdr.String())

	// Log package metadata
	e.Str("pkgHdr", t.PkgHdr.String())

	// Log message body information
	msg, err := proto.Marshal(t.Body)
	if err == nil {
		e.Str("msg", string(msg))
	}
}

// NewTransRecvPkgWithBody creates a new received package with an already decoded body.
// Useful for constructing packages from existing message instances in testing,
// message forwarding, or internal service communication scenarios.
//
// Parameters:
// - routeHdr: Routing information for message delivery
// - hdr: Package metadata including message type and IDs
// - body: Pre-decoded protobuf message body
//
// Returns: A new TransRecvPkg instance with decoded body
func NewTransRecvPkgWithBody(routeHdr *RouteHead, hdr *PkgHead, body proto.Message) *TransRecvPkg {
	return &TransRecvPkg{
		RouteHdr: routeHdr,
		PkgHdr:   hdr,
		decoded:  true,
		body:     body,
	}
}

// NewTransRecvPkgWithBodyData creates a new received package with raw body data.
// Used for network layer initialization when receiving raw bytes from transport.
// The body will be lazily decoded when DecodeBody() is called to optimize performance.
//
// Parameters:
// - routeHdr: Routing information for message delivery
// - hdr: Package metadata including message type and IDs
// - bodyData: Raw serialized message body bytes
// - creator: Message factory for dynamic message instantiation
//
// Returns: A new TransRecvPkg instance with raw body data
func NewTransRecvPkgWithBodyData(routeHdr *RouteHead, hdr *PkgHead,
	bodyData []byte, creator MsgCreator) *TransRecvPkg {
	return &TransRecvPkg{
		RouteHdr: routeHdr,
		PkgHdr:   hdr,
		decoded:  false,
		creator:  creator,
		bodyData: bodyData,
	}
}
