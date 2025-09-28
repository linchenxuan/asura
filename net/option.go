// Package net provides network communication components for the Asura framework,
// including message routing, dispatching, and transport mechanisms.
package net

// TransPkgOption defines a functional option type for configuring TransSendPkg instances.
// This pattern allows for flexible and extensible configuration of message packages.
//
// Usage example:
// pkg := NewResPkg(msgID, body, WithSrcActorID(123), WithRetCode(0))
type TransPkgOption func(*TransSendPkg)

// WithSrcActorID sets the source actor ID in the package header.
// This is used to identify which actor sent the message, useful for
// tracking message origins in a distributed system.
//
// Parameters:
// - id: The unique identifier of the source actor
//
// Returns:
// A TransPkgOption function that applies the source actor ID to a TransSendPkg
func WithSrcActorID(id uint64) TransPkgOption {
	return func(pkg *TransSendPkg) {
		pkg.PkgHdr.SrcActorID = id
	}
}

// WithName sets the message ID in the package header.
// The message ID is used to identify the type of message being sent,
// allowing the receiving end to properly route and handle it.
//
// Parameters:
// - msgID: A string identifier for the message type
//
// Returns:
// A TransPkgOption function that applies the message ID to a TransSendPkg
func WithName(msgID string) TransPkgOption {
	return func(pkg *TransSendPkg) {
		pkg.PkgHdr.MsgID = msgID
	}
}

// WithAreaID sets the area ID in the routing header.
// Area ID is used for regional partitioning of services,
// ensuring messages are routed to the correct geographical or logical area.
//
// Parameters:
// - areaID: The identifier for the target area
//
// Returns:
// A TransPkgOption function that applies the area ID to a TransSendPkg's routing header
func WithAreaID(areaID uint32) TransPkgOption {
	return func(pkg *TransSendPkg) {
		if pkg != nil && pkg.GetRouteHdr() != nil {
			switch v := pkg.GetRouteHdr().GetRouteType().(type) {
			case *RouteHead_Rand:
				v.Rand.AreaID = areaID
			case *RouteHead_BroadCast:
				v.BroadCast.AreaID = areaID
			}
		}
	}
}

// WithRetCode sets the return code in the package header.
// Return codes are used to indicate the result of a request,
// with 0 typically representing success and other values representing errors.
//
// Parameters:
// - ret: The return code value
//
// Returns:
// A TransPkgOption function that applies the return code to a TransSendPkg
func WithRetCode(ret int32) TransPkgOption {
	return func(pkg *TransSendPkg) {
		pkg.PkgHdr.RetCode = ret
	}
}

// WithSetVersion sets the destination set version in the routing header.
// Set versioning is used for service sharding and migration scenarios,
// allowing messages to be routed to specific versions of service sets.
//
// Parameters:
// - setVersion: The version identifier for the target service set
//
// Returns:
// A TransPkgOption function that applies the set version to a TransSendPkg's routing header
func WithSetVersion(setVersion uint64) TransPkgOption {
	return func(pkg *TransSendPkg) {
		if pkg != nil && pkg.RouteHdr != nil {
			pkg.RouteHdr.DstSetVersion = setVersion
		}
	}
}

// TransportOption defines configuration parameters for initializing a Transport instance.
// This struct provides the necessary components for message processing and dispatching.
//
// Usage example:
//
//	transport.Start(TransportOption{
//	    Creator: messageCreator,
//	    Handler: messageHandler
//	})
type TransportOption struct {
	// Creator is responsible for creating message instances from binary data
	// based on message IDs. It is used by the transport layer to instantiate
	// the correct message type when receiving data.
	Creator MsgCreator

	// Handler receives deserialized messages and dispatches them to the appropriate
	// processing logic in the application layer. It serves as the callback interface
	// between the transport layer and the application logic.
	Handler DispatcherReceiver
}
