// Package net provides core networking infrastructure for Asura game server framework.
// It handles message routing, dispatching, and transport between game server components.
//
// This file implements the message dispatching system, which serves as the central
// hub for processing and distributing messages within the game server architecture.
// The dispatcher manages message flow between transport layers and message processing
// layers, implementing rate limiting and filtering capabilities.
//
// Key components include message delivery containers, layer receivers, configuration
// structures, and the main dispatcher implementation that coordinates all message
// handling activities.
//
// The dispatcher is designed to support high-throughput, low-latency message processing
// in distributed game server environments, with built-in support for rate limiting,
// message filtering, and multi-layer message processing.
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
package net

import (
	"errors"
	"fmt"
	"sync"

	"github.com/lcx/asura/config"
	"github.com/lcx/asura/log"
)

// DispatcherDelivery extends TransportDelivery to include protocol information and response options
// Used as the primary container for message delivery through the dispatcher system
// Combines transport-level delivery information with protocol-specific metadata
//
// This structure serves as the bridge between the transport layer and message processing layers,
// providing access to both raw message data and parsed protocol information. It includes utility
// methods for accessing common message properties and sending error responses.
//
// Fields:
// - TransportDelivery: Embedded transport delivery containing raw package data
// - ProtoInfo: Protocol metadata for the current message
// - ResOpts: Optional parameters for response message construction
//
// Methods:
// - GetCurReq: Retrieves the current request package
// - GetReqSrcActorID: Gets the sender's actor ID from the request package
// - GetReqDstActorID: Gets the receiver's actor ID from the request package
// - GetSrcEntityID: Gets the source entity ID from the request package
// - GetSrcSetVersion: Gets the source server set version from the request package
// - GetSrcClientVersion: Gets the source client version from the request package
// - sendBackErr: Sends an error response with specified error code
//
// Usage example:
//
//	func OnRecvDispatcherPkg(dd *DispatcherDelivery) error {
//	    reqID := dd.GetCurReq().PkgHdr.GetMsgID()
//	    source := dd.GetReqSrcActorID()
//	    // Process message...
//	    if err != nil {
//	        return dd.sendBackErr(ErrorCode)
//	    }
//	    return nil
//	}
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
type DispatcherDelivery struct {
	*TransportDelivery                  // Embedded transport delivery containing raw package data
	ProtoInfo          *MsgProtoInfo    // Protocol metadata for the current message
	ResOpts            []TransPkgOption // Optional parameters for response message construction
}

// GetReqSrcActorID gets the sender's actor ID from the request package
// Returns: The source actor ID from the package header
// Used for identifying the sender in distributed server architecture
func (dd *DispatcherDelivery) GetReqSrcActorID() uint64 {
	return dd.TransportDelivery.Pkg.PkgHdr.GetSrcActorID()
}

// GetReqDstActorID gets the receiver's actor ID from the request package
// Returns: The destination actor ID from the package header
// Used for identifying the intended recipient in distributed server architecture
func (dd *DispatcherDelivery) GetReqDstActorID() uint64 {
	return dd.TransportDelivery.Pkg.PkgHdr.GetDstActorID()
}

// GetSrcEntityID gets the source entity ID from the request package
// Returns: The source entity ID from the route header
// Typically represents a player or game object ID
func (dd *DispatcherDelivery) GetSrcEntityID() uint32 {
	return dd.TransportDelivery.Pkg.RouteHdr.GetSrcEntityID()
}

// GetSrcSetVersion gets the source server set version from the request package
// Returns: The source server set version from the route header
// Used for versioning and compatibility checks between server components
func (dd *DispatcherDelivery) GetSrcSetVersion() uint64 {
	return dd.TransportDelivery.Pkg.RouteHdr.GetSrcSetVersion()
}

// GetSrcClientVersion gets the source client version from the request package
// Returns: The source client version from the package header
// Used for client compatibility checks and version-dependent processing
func (dd *DispatcherDelivery) GetSrcClientVersion() int64 {
	return dd.TransportDelivery.Pkg.PkgHdr.GetSrcCltVersion()
}

// MsgLayerReceiver defines the interface for message layer receivers
// Implementations handle dispatcher packages at specific processing layers
// Core interface for message processing within the dispatcher system
//
// This interface is implemented by different message processing layers
// (stateless, stateful, async) to receive and handle messages dispatched
// by the central dispatcher. Each implementation provides specific processing
// logic appropriate for its layer type.
//
// Methods:
//   - OnRecvDispatcherPkg: Processes a dispatched message package
//     Parameters:
//   - dd: The dispatcher delivery containing the message and metadata
//     Returns: Error if processing fails, nil otherwise
//
// Usage example:
// type MyMsgHandler struct {}
//
//	func (h *MyMsgHandler) OnRecvDispatcherPkg(dd *DispatcherDelivery) error {
//	    // Process message based on message type, source, etc.
//	    // May send response using dd.TransSendBack
//	    return nil
//	}
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
type MsgLayerReceiver interface {
	// OnRecvDispatcherPkg processes a dispatched message package
	// Parameters:
	// - dd: The dispatcher delivery containing the message and metadata
	// Returns: Error if processing fails, nil otherwise
	// Core method for message processing in the dispatcher system
	OnRecvDispatcherPkg(*DispatcherDelivery) error
}

// MsgFilterPluginCfg defines configuration for the message filter plugin.
// It contains a list of message IDs that should be filtered by the dispatcher.
//
// This configuration structure is typically populated from application settings
// and used to control which messages should be intercepted by the filtering mechanism.
type MsgFilterPluginCfg struct {
	MsgFilter []string `mapstructure:"msgFilter"`
}

// GetName returns the configuration name for MsgFilterPluginCfg
func (c *MsgFilterPluginCfg) GetName() string {
	return "msg_filter"
}

// Validate validates the MsgFilterPluginCfg parameters
func (c *MsgFilterPluginCfg) Validate() error {
	// MsgFilter can be empty (no filtering) or contain valid message IDs
	// No specific validation needed for the filter list itself
	return nil
}

// DispatcherConfig contains configuration parameters for the dispatcher
// Uses token bucket algorithm for rate limiting instead of the previous funnel algorithm
// Controls message processing behavior, rate limits, and filtering options
//
// This structure configures the dispatcher's behavior, including rate limiting settings
// and message filtering rules. It uses a token bucket algorithm for more flexible rate
// limiting compared to the older funnel algorithm implementation.
//
// Fields:
// - RecvRateLimit: Maximum number of messages to receive per second (supports hot reloading)
// - TokenBurst: Initial token bucket size for burst handling
// - MsgFilter: Configuration for message filtering plugin
//
// Used in the NewDispatcher constructor to initialize the dispatcher with specific settings.
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
type DispatcherConfig struct {
	RecvRateLimit int                `mapstructure:"recvRateLimit"`
	TokenBurst    int                `mapstructure:"tokenBurst"`
	MsgFilter     MsgFilterPluginCfg `mapstructure:"msgFilter"`
}

// GetName returns the configuration name for DispatcherConfig
func (c *DispatcherConfig) GetName() string {
	return "dispatcher"
}

// Validate validates the DispatcherConfig parameters
func (c *DispatcherConfig) Validate() error {
	if c.RecvRateLimit <= 0 {
		return fmt.Errorf("RecvRateLimit must be positive")
	}
	if c.TokenBurst <= 0 {
		return fmt.Errorf("TokenBurst must be positive")
	}
	if c.RecvRateLimit > 1000000 {
		return fmt.Errorf("RecvRateLimit cannot exceed 1,000,000 messages per second")
	}
	if c.TokenBurst > c.RecvRateLimit*10 {
		return fmt.Errorf("TokenBurst cannot exceed 10 times RecvRateLimit")
	}
	return nil
}

// Dispatcher is the central message processing hub in the networking system
// Manages message flow between transport layers and message processing layers
// Implements rate limiting, filtering, and message routing capabilities
//
// This is the core component of the message processing system, responsible for
// coordinating the flow of messages from transport layers to appropriate message
// processing layers. It implements rate limiting to protect against overload,
// filtering to allow or block specific messages, and routing to direct messages
// to the correct processing layer.
//
// The dispatcher follows a modular design that allows for flexible configuration
// of transport layers, message processing layers, and filters.
//
// Fields:
// - msglayers: Map of message layer types to their respective receivers
// - transports: List of registered transport layers
// - recvLimiter: Rate limiter implementation using token bucket algorithm
// - filters: Chain of filters applied to incoming messages
// - msgFilterMap: Map of filtered message IDs
// - msgMgr: Message manager for protocol information lookup
//
// Methods:
// - NewDispatcher: Creates a new dispatcher instance
// - RegisterMsglayer: Registers a message layer receiver
// - StartServe: Starts the dispatcher service
// - RegDispatcherFilter: Registers additional dispatcher filters
// - OnRecvTransportPkg: Implementation of DispatcherReceiver interface
// - handleTransportMsgImpl: Internal method for message handling
// - chooseMsgLayerReceiver: Selects appropriate message layer receiver
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
type Dispatcher struct {
	msglayers    map[MsgLayerType]MsgLayerReceiver // Map of message layer types to their handlers
	transports   []Transport                       // Registered transport layers
	recvLimiter  *DispatcherRecvLimiter            // Rate limiter implementation
	filters      DispatcherFilterChain             // Chain of message filters
	msgFilterMap map[string]struct{}               // Map of filtered message IDs
	msgMgr       *MessageManager                   // Message manager for protocol info lookup
	config       *DispatcherConfig                 // Configuration for the dispatcher
	lock         sync.RWMutex                      // Protects configuration updates
}

// NewDispatcher creates a new dispatcher instance with specified configuration
// Parameters:
// - cfg: Dispatcher configuration settings (may be nil to use defaults)
// - msgMgr: Message manager for protocol information lookup
// - trans: List of transport layers to register with the dispatcher
// Returns:
// - *Dispatcher: Newly created dispatcher instance
// - error: Error if initialization fails
//
// Constructor for the dispatcher that initializes with default or provided configuration,
// validates parameters, and sets up internal components including rate limiter, filters,
// and message filter map.
//
// Usage example:
// msgMgr := NewManager()
// transports := []Transport{tcpTransport, wsTransport}
// dispatcher, err := NewDispatcher(&DispatcherConfig{RecvRateLimit: 10000}, msgMgr, transports)
//
//	if err == nil {
//	    // Use dispatcher...
//	}
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func NewDispatcher(cfg *DispatcherConfig, msgMgr *MessageManager, trans []Transport) (*Dispatcher, error) {
	if cfg == nil {
		return nil, errors.New("DispatcherConfig cannot be nil, use NewDispatcherWithConfigManager for dynamic configuration")
	}

	d := &Dispatcher{
		msglayers:    make(map[MsgLayerType]MsgLayerReceiver),
		transports:   trans,
		recvLimiter:  NewTokenRecvLimiter(cfg.RecvRateLimit, cfg.TokenBurst),
		msgFilterMap: make(map[string]struct{}),
		msgMgr:       msgMgr,
		config:       cfg,
		lock:         sync.RWMutex{},
	}

	d.reloadMsgFilterCfg(&cfg.MsgFilter)

	d.filters = append(d.filters, d.msgFilter)
	d.filters = append(d.filters, d.recvLimiter.recvLimiterFilter)

	return d, nil
}

// NewDispatcherWithConfigManager creates a dispatcher that supports configuration hot-reload.
// This constructor initializes the dispatcher with configuration from the config manager
// and registers it as a configuration change listener for dynamic updates.
func NewDispatcherWithConfigManager(configManager config.ConfigManager, msgMgr *MessageManager, trans []Transport) (*Dispatcher, error) {
	if configManager == nil {
		return nil, errors.New("configManager cannot be nil")
	}

	// Load configuration from config manager
	cfg := &DispatcherConfig{}
	if err := configManager.LoadConfig("dispatcher", cfg); err != nil {
		return nil, fmt.Errorf("failed to load dispatcher config: %w", err)
	}

	d := &Dispatcher{
		msglayers:    make(map[MsgLayerType]MsgLayerReceiver),
		transports:   trans,
		recvLimiter:  NewTokenRecvLimiter(cfg.RecvRateLimit, cfg.TokenBurst),
		msgFilterMap: make(map[string]struct{}),
		msgMgr:       msgMgr,
		config:       cfg,
		lock:         sync.RWMutex{},
	}

	d.reloadMsgFilterCfg(&cfg.MsgFilter)

	d.filters = append(d.filters, d.msgFilter)
	d.filters = append(d.filters, d.recvLimiter.recvLimiterFilter)

	// Register as configuration change listener
	configManager.AddChangeListener(d)

	return d, nil
}

// OnConfigChanged implements the ConfigChangeListener interface for Dispatcher.
// This method is called when the dispatcher configuration is updated in the config manager.
// It handles dynamic updates to dispatcher settings without requiring service restart.
func (d *Dispatcher) OnConfigChanged(configName string, newConfig, oldConfig config.Config) error {
	if configName != "dispatcher" {
		return nil
	}

	newCfg, ok := newConfig.(*DispatcherConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type for Dispatcher")
	}

	// Validate the new configuration
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("invalid dispatcher configuration: %w", err)
	}

	// Update configuration atomically
	d.lock.Lock()
	defer d.lock.Unlock()

	// Update rate limiter with new configuration
	d.recvLimiter.Reload(newCfg.RecvRateLimit, newCfg.TokenBurst)

	// Update message filter configuration
	d.reloadMsgFilterCfg(&newCfg.MsgFilter)

	// Update configuration reference
	d.config = newCfg

	log.Info().Str("configName", configName).Msg("Dispatcher configuration updated successfully")
	return nil
}

// GetConfigName implements the ConfigChangeListener interface for Dispatcher.
// Returns the configuration name that this listener is interested in.
func (d *Dispatcher) GetConfigName() string {
	return "dispatcher"
}

// RegisterMsglayer registers a message layer receiver with the dispatcher
// Parameters:
// - t: Message layer type (stateless, stateful, async)
// - m: Message layer receiver implementation
// Returns: Error if registration fails, nil otherwise
//
// Registers a message handler for a specific message layer type. Each layer type
// can have only one handler registered at a time. The layer type must be one of
// the valid predefined types (MsgLayerType_Stateless, MsgLayerType_Stateful,
// MsgLayerType_Async).
//
// Usage example:
// statelessHandler := &StatelessMsgHandler{}
// err := dispatcher.RegisterMsglayer(MsgLayerType_Stateless, statelessHandler)
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func (d *Dispatcher) RegisterMsglayer(t MsgLayerType, m MsgLayerReceiver) error {
	if m == nil {
		return errors.New("RegisterMsglayer receiver is nil")
	}
	if t <= MsgLayerType_None || t >= MsgLayerType_Max {
		return errors.New("RegisterMsglayer invalid")
	}

	if _, ok := d.msglayers[t]; ok {
		return errors.New("RegisterMsglayer duplicated")
	}
	d.msglayers[t] = m
	return nil
}

// StartServe starts the dispatcher service and initializes transport layers
// Parameters:
// - creater: Message creator for decoding incoming messages
// Returns: Error if startup fails, nil otherwise
//
// Starts all registered transport layers with the provided message creator
// and dispatcher as the handler. If any transport fails to start, all
// previously started transports are stopped before returning an error.
//
// This method is typically called during server initialization to begin
// message processing operations.
//
// Usage example:
// msgCreator := &MyMsgCreator{}
// err := dispatcher.StartServe(msgCreator)
//
//	if err != nil {
//	    // Handle startup failure
//	}
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func (d *Dispatcher) StartServe(creater MsgCreator) error {
	to := TransportOption{
		Creator: creater,
		Handler: d,
	}
	succTransports := make([]Transport, 0, len(d.transports))
	for _, t := range d.transports {
		if err := t.Start(to); err != nil {
			// 启动异常时，需要先将已经start的transport停掉
			for _, succt := range succTransports {
				if err := succt.Stop(); err != nil {
				}
			}
			return errors.New("Start")
		}
		succTransports = append(succTransports, t)
	}

	return nil
}

// RegDispatcherFilter registers an additional filter with the dispatcher
// Parameters:
// - f: Dispatcher filter implementation
//
// Adds a custom filter to the dispatcher's filter chain. Filters are applied
// in the order they are registered. Custom filters can implement various
// processing logic such as authentication, validation, logging, or metrics collection.
//
// Usage example:
// authFilter := &AuthFilter{}
// dispatcher.RegDispatcherFilter(authFilter)
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func (d *Dispatcher) RegDispatcherFilter(f DispatcherFilter) {
	d.filters = append(d.filters, f)
}

// OnRecvTransportPkg implements the DispatcherReceiver interface
// Parameters:
// - td: Transport delivery containing the incoming message
// Returns: Error if processing fails, nil otherwise
//
// Callback method called by transport layers when a new message is received.
// Retrieves protocol information for the message, creates a dispatcher delivery,
// and passes it through the filter chain for processing.
//
// This is the entry point for all incoming messages into the dispatcher system.
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func (d *Dispatcher) OnRecvTransportPkg(td *TransportDelivery) error {
	info, _ := d.msgMgr.GetProtoInfo(td.Pkg.PkgHdr.GetMsgID())

	dd := &DispatcherDelivery{
		TransportDelivery: td,
		ProtoInfo:         info,
	}

	return d.filters.Handle(dd, d.handleTransportMsgImpl)
}

// handleTransportMsgImpl is the internal method for message handling after filtering
// Parameters:
// - dd: Dispatcher delivery containing the message and metadata
// Returns: Error if handling fails, nil otherwise
//
// Called by the filter chain after all filters have been applied. Selects the
// appropriate message layer receiver based on the message's layer type and
// dispatches the message to it for processing.
//
// Internal method not intended to be called directly by external code.
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func (d *Dispatcher) handleTransportMsgImpl(dd *DispatcherDelivery) error {
	m := d.chooseMsgLayerReceiver(dd)
	if m == nil {
		return errors.New("choose msglayer failed")
	}

	return m.OnRecvDispatcherPkg(dd)
}

// chooseMsgLayerReceiver selects the appropriate message layer receiver for a message
// Parameters:
// - dd: Dispatcher delivery containing the message and metadata
// Returns: MsgLayerReceiver implementation for the message's layer type, or nil if none
//
// Looks up and returns the registered message layer receiver based on the
// message's protocol information. If no receiver is registered for the
// message's layer type, returns nil.
//
// Internal method used by handleTransportMsgImpl to determine message routing.
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func (d *Dispatcher) chooseMsgLayerReceiver(dd *DispatcherDelivery) MsgLayerReceiver {
	if m, ok := d.msglayers[dd.ProtoInfo.MsgLayerType]; ok {
		return m
	}
	return nil
}

// checkParamValid validates dispatcher configuration parameters
// Parameters:
// - cfg: Dispatch configuration to validate
// Returns: Error if any parameter is invalid, nil otherwise
//
// Validates that required configuration parameters are within acceptable ranges.
// Ensures that rate limit and token burst values are positive integers.
//
// Internal helper function used by NewDispatcher to validate configuration.
//
// Reference: <mcfile name="dispatcher.go" path="/root/asura/net/dispatcher.go"></mcfile>
func checkParamValid(cfg *DispatcherConfig) error {
	if cfg.RecvRateLimit <= 0 {
		return errors.New("checkParamValid RecvRateLimit == 0")
	}
	if cfg.TokenBurst <= 0 {
		return errors.New("checkParamValid TokenBurst == 0")
	}
	return nil
}
