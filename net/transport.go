// Package net implements core networking components for the Asura distributed game server framework.
// It handles message packaging, routing, and serialization for inter-service communication
// in a high-performance, low-latency gaming environment.
package net

// Transport defines the core interface for all network transport components in the Asura framework.
// It provides lifecycle management methods for starting, stopping receiving, and fully stopping transport services.
// This interface serves as the foundation for both client-server (CS) and server-server (SS) communication layers.
type Transport interface {
	// Start initializes and starts the transport service with the provided options.
	// Responsible for setting up network connections, initializing resources, and starting event loops.
	// Returns an error if initialization or startup fails.
	Start(TransportOption) error

	// StopRecv gracefully stops receiving new messages while allowing existing messages to be processed.
	// Useful for controlled shutdown scenarios where in-flight messages should be completed.
	// Returns an error if stopping the receive functionality fails.
	StopRecv() error

	// Stop fully shuts down the transport service, closing all connections and releasing resources.
	// May terminate in-flight operations depending on implementation.
	// Returns an error if the shutdown process encounters issues.
	Stop() error
}

// CSTransport extends the base Transport interface with client-specific messaging capabilities.
// Designed for communication between game servers and clients in distributed game architectures.
type CSTransport interface {
	Transport

	// SendToClient sends a streaming message to a specific client.
	// Handles client-specific protocol formatting, session management, and delivery guarantees.
	// Parameters:
	//   pkg - The transport package containing the message to be sent
	// Returns an error if message delivery fails.
	SendToClient(pkg TransSendPkg) error
}

// SSTransport extends the base Transport interface with server-to-server communication capabilities.
// Used for inter-service messaging in distributed game server clusters.
type SSTransport interface {
	Transport

	// SendToServer sends a streaming message to a specific server in the cluster.
	// Handles server discovery, routing, and high-performance inter-process communication.
	// Parameters:
	//   pkg - The transport package containing the message to be sent
	// Returns an error if message delivery fails.
	SendToServer(pkg TransSendPkg) error
}

// SendBackFunc defines a function type used for sending response messages back to clients or servers.
// Provides a standardized way for upper-layer logic to respond to incoming messages.
// Parameters:
//
//	pkg - Pointer to the transport package containing the response message
//
// Returns an error if message sending fails.
type SendBackFunc func(pkg *TransSendPkg) error

// TransportDelivery encapsulates a received message along with metadata and handling instructions.
// Used to deliver messages from the transport layer to higher-level components like dispatchers.
type TransportDelivery struct {
	// TransSendBack provides a function for sending responses back through the same transport channel.
	TransSendBack SendBackFunc

	// Pkg contains the actual transport package with message content and headers.
	Pkg *TransRecvPkg
}

// DispatcherReceiver defines the callback interface for handling received transport packages.
// Implemented by dispatchers to receive messages from the transport layer.
type DispatcherReceiver interface {
	// OnRecvTransportPkg is called when a new transport package is received.
	// Responsible for processing or routing the message to appropriate handlers.
	// Parameters:
	//   td - The transport delivery containing the message and metadata
	// Returns an error if message processing fails.
	OnRecvTransportPkg(td *TransportDelivery) error
}
