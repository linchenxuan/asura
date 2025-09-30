package net

import "time"

// NtfPkgSender defines the interface for sending notification messages.
// Supports both client and server notification patterns commonly used
// in real-time gaming applications for state updates and events.
type NtfPkgSender interface {
	// NtfClient sends a notification package to a client in the distributed game server.
	// Used for pushing game state updates, events, or system messages to connected clients.
	NtfClient(pkg TransSendPkg)

	// NtfServer sends a notification package to another server in the distributed system.
	// Enables server-to-server communication for broadcasting state changes or triggering
	// cross-service operations in real-time game environments.
	NtfServer(pkg TransSendPkg)
}

// RPCPkgSender extends notification capabilities with synchronous RPC support.
// Provides request-response patterns for game logic requiring immediate feedback
// such as player actions, state queries, or transactional operations.
type RPCPkgSender interface {
	NtfPkgSender

	// RPCServer performs a synchronous RPC call to another server and waits for the response.
	// Used for critical game operations that require immediate validation or data retrieval
	// across distributed services.
	RPCServer(pkg TransSendPkg) (resPkg TransRecvPkg, _ error)

	// GetRPCDeadline retrieves the deadline for pending RPC operations and a boolean
	// indicating if a deadline is set. Helps implement timeout mechanisms for RPC calls
	// to prevent blocking indefinitely in high-concurrency game server environments.
	GetRPCDeadline() (time.Time, bool)
}

// ARPCPkgSender defines the interface for asynchronous RPC message sending.
// Enables non-blocking communication patterns where immediate response isn't required,
// improving performance in high-throughput gaming scenarios.
type ARPCPkgSender interface {
	// AsyncRPCToServer sends an asynchronous RPC request to another server and provides
	// a callback function to handle the response when it arrives. This allows the calling
	// service to continue processing without blocking, ideal for operations that don't
	// require immediate results such as analytics data submission or non-critical updates.
	AsyncRPCToServer(func(TransRecvPkg, error), TransSendPkg)
}

// DeferPkgSender provides interfaces for deferred notification message sending.
// Allows queuing notifications that should be sent after the current response has been
// processed, maintaining message ordering guarantees in complex game interactions.
type DeferPkgSender interface {
	// DeferNtfClient queues a notification to be sent to a client after the current
	// response package has been dispatched. Useful for chaining related updates
	// while maintaining proper message ordering for client-side processing.
	DeferNtfClient(pkg TransSendPkg)

	// DeferNtfServer queues a notification to be sent to another server after the current
	// response package has been processed. Ensures proper sequence of operations across
	// distributed services in complex game workflows.
	DeferNtfServer(pkg TransSendPkg)
}

// PkgSender provides a comprehensive interface for all types of message sending operations
// in the distributed game server framework. Combines notification, synchronous RPC,
// and asynchronous RPC capabilities into a single unified interface.
type PkgSender interface {
	// NtfClient sends a notification package to a client in the distributed game server.
	// Returns an error if the notification cannot be queued or dispatched.
	NtfClient(pkg TransSendPkg) error

	// NtfServer sends a notification package to another server in the distributed system.
	// Returns an error if the notification cannot be queued or dispatched.
	NtfServer(pkg TransSendPkg) error

	// RPCServer performs a synchronous RPC call to another server and waits for the response.
	// Returns the response package or an error if the RPC fails or times out.
	RPCServer(pkg TransSendPkg) (TransRecvPkg, error)

	// AsyncRPCToServer sends an asynchronous RPC request to another server and provides
	// a callback function to handle the response when it arrives. Returns an error if
	// the asynchronous request cannot be queued for processing.
	AsyncRPCToServer(func(TransRecvPkg, error), TransSendPkg) error
}
