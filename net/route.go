// Package net provides the core networking functionality for the Asura distributed game server framework.
// It handles message routing, dispatching, and transport between game server components.
package net

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"time"
)

// Route defines the interface for routing strategies used to determine message destinations
// in distributed game server architectures. Implementations provide specific routing logic
// for different message distribution patterns.
type Route interface {
	// GetRouteKey returns a unique string key used for consistent routing decisions
	// across distributed nodes. This key should be deterministic for the same route parameters.
	GetRouteKey() string

	// GetRouteStrategy returns the routing strategy type identifier
	// which determines the routing algorithm to be applied.
	GetRouteStrategy() RouteStrategy

	// Validate checks if the route configuration is valid and complete
	// Returns error if any required parameters are missing or invalid.
	Validate() error

	// String returns a human-readable description of the route configuration
	// for debugging and logging purposes.
	String() string
}

// RouteStrategy represents the enumeration of supported routing strategies
// in the distributed game server framework.
type RouteStrategy int

const (
	// RouteP2P indicates point-to-point routing for direct entity communication
	RouteP2P RouteStrategy = iota + 1

	// RouteRand indicates random routing for load balancing across multiple nodes
	RouteRand

	// RouteHash indicates hash-based routing for consistent key distribution
	RouteHash

	// RouteBroadCast indicates broadcast routing to all nodes in specified area
	RouteBroadCast

	// RouteMulCast indicates multicast routing to specific entity list
	RouteMulCast

	// RouteInter indicates internal service communication routing
	RouteInter
)

// String returns the string representation of RouteStrategy for logging and debugging
func (r RouteStrategy) String() string {
	switch r {
	case RouteP2P:
		return "P2P"
	case RouteRand:
		return "Random"
	case RouteHash:
		return "Hash"
	case RouteBroadCast:
		return "Broadcast"
	case RouteMulCast:
		return "Multicast"
	case RouteInter:
		return "Internal"
	default:
		return "Unknown"
	}
}

// RouteTypeP2P implements point-to-point routing for direct entity communication
// Used when a message needs to be delivered to a specific entity identified by ID
// in high-performance game server scenarios requiring low latency communication.
type RouteTypeP2P struct {
	// DstEntityID is the unique identifier of the target entity
	DstEntityID uint64

	// Precomputed route key for performance optimization
	routeKey string
}

// NewRouteTypeP2P creates a new P2P route with precomputed route key
// Parameters:
// - entityID: The unique identifier of the target entity
// Returns: A new RouteTypeP2P instance with precomputed routing information
func NewRouteTypeP2P(entityID uint64) *RouteTypeP2P {
	return &RouteTypeP2P{
		DstEntityID: entityID,
		routeKey:    fmt.Sprintf("p2p:%d", entityID),
	}
}

// GetRouteKey returns the precomputed routing key
// Returns: A string key that uniquely identifies this route configuration
func (r *RouteTypeP2P) GetRouteKey() string {
	return r.routeKey
}

// GetRouteStrategy returns the P2P routing strategy identifier
// Returns: The RouteStrategy constant for P2P routing
func (r *RouteTypeP2P) GetRouteStrategy() RouteStrategy {
	return RouteP2P
}

// Validate checks if the destination entity ID is valid
// Returns: nil if valid, error if the entity ID is invalid (zero)
func (r *RouteTypeP2P) Validate() error {
	if r.DstEntityID == 0 {
		return fmt.Errorf("invalid entity ID: %d", r.DstEntityID)
	}
	return nil
}

// String returns a human-readable description of the P2P route
// Returns: Formatted string with routing information for debugging
func (r *RouteTypeP2P) String() string {
	return fmt.Sprintf("P2P[EntityID:%d]", r.DstEntityID)
}

// RouteTypeRand implements random routing strategy for load balancing
// Supports both pure random and consistent routing based on constant key
// Optimized for distributing requests evenly across multiple server instances.
type RouteTypeRand struct {
	// FuncID identifies the service function for service discovery
	FuncID uint32

	// AreaID limits the routing scope to specific area
	AreaID uint32

	// UseConstantRoute enables consistent routing based on ConstantKey
	UseConstantRoute bool

	// ConstantKey provides the routing key for consistent routing
	ConstantKey uint64

	// Precomputed route key for performance optimization
	routeKey string
}

// NewRouteTypeRand creates a new random route with precomputed route key
// Parameters:
// - funcID: The service function identifier
// - areaID: The area identifier to limit routing scope
// - useConstant: Whether to use consistent routing based on constant key
// - constantKey: The constant key for consistent routing
// Returns: A new RouteTypeRand instance configured with the specified parameters
func NewRouteTypeRand(funcID uint32, areaID uint32, useConstant bool, constantKey uint64) *RouteTypeRand {
	r := &RouteTypeRand{
		FuncID:           funcID,
		AreaID:           areaID,
		UseConstantRoute: useConstant,
		ConstantKey:      constantKey,
	}

	if useConstant {
		r.routeKey = fmt.Sprintf("rand:%d:%d", funcID, constantKey)
	} else {
		r.routeKey = fmt.Sprintf("rand:%d:%d:random", funcID, areaID)
	}

	return r
}

// GetRouteKey returns the precomputed routing key
// Returns: A string key that uniquely identifies this route configuration
func (r *RouteTypeRand) GetRouteKey() string {
	return r.routeKey
}

// GetRouteStrategy returns the random routing strategy identifier
// Returns: The RouteStrategy constant for random routing
func (r *RouteTypeRand) GetRouteStrategy() RouteStrategy {
	return RouteRand
}

// Validate checks if the function ID is valid
// Returns: nil if valid, error if the function ID is invalid (zero)
func (r *RouteTypeRand) Validate() error {
	if r.FuncID == 0 {
		return fmt.Errorf("invalid func ID: %d", r.FuncID)
	}
	return nil
}

// String returns a human-readable description of the random route
// Returns: Formatted string with routing information for debugging
func (r *RouteTypeRand) String() string {
	if r.UseConstantRoute {
		return fmt.Sprintf("Rand[Func:%d,Key:%d]", r.FuncID, r.ConstantKey)
	}
	return fmt.Sprintf("Rand[Func:%d,Area:%d]", r.FuncID, r.AreaID)
}

// RouteTypeHash implements hash-based routing using FNV-1a algorithm
// Provides consistent routing for the same hash key across distributed nodes
// Optimized for session stickiness and sharded data access patterns.
type RouteTypeHash struct {
	// FuncID identifies the service function for routing
	FuncID uint32

	// HashKey is the string key used for hash calculation
	HashKey string

	// AreaID limits the routing scope to specific area
	AreaID uint32

	// Replica specifies the number of replicas for consistent hashing
	Replica int

	// Precomputed route key for performance optimization
	routeKey string
}

// NewRouteTypeHash creates a new hash route with precomputed route key
// Parameters:
// - funcID: The service function identifier
// - hashKey: The string key for hash calculation
// - areaID: The area identifier to limit routing scope
// - replica: The number of replicas for consistent hashing
// Returns: A new RouteTypeHash instance with precomputed hash and routing information
func NewRouteTypeHash(funcID uint32, hashKey string, areaID uint32, replica int) *RouteTypeHash {
	h := fnv.New64a()
	h.Write([]byte(hashKey))
	hash := h.Sum64()

	return &RouteTypeHash{
		FuncID:   funcID,
		HashKey:  hashKey,
		AreaID:   areaID,
		Replica:  replica,
		routeKey: fmt.Sprintf("hash:%d:%d:%d:%d", funcID, areaID, hash, replica),
	}
}

// GetRouteKey returns the precomputed routing key
// Returns: A string key that uniquely identifies this route configuration
func (r *RouteTypeHash) GetRouteKey() string {
	return r.routeKey
}

// GetRouteStrategy returns the hash routing strategy identifier
// Returns: The RouteStrategy constant for hash-based routing
func (r *RouteTypeHash) GetRouteStrategy() RouteStrategy {
	return RouteHash
}

// Validate checks if the function ID and hash key are valid
// Returns: nil if valid, error if function ID is zero or hash key is empty
func (r *RouteTypeHash) Validate() error {
	if r.FuncID == 0 {
		return fmt.Errorf("invalid func ID: %d", r.FuncID)
	}
	if r.HashKey == "" {
		return fmt.Errorf("empty hash key")
	}
	return nil
}

// String returns a human-readable description of the hash route
// Returns: Formatted string with routing information for debugging
func (r *RouteTypeHash) String() string {
	return fmt.Sprintf("Hash[Func:%d,Key:%s,Area:%d]", r.FuncID, r.HashKey, r.AreaID)
}

// RouteTypeBroadCast implements broadcast routing to all nodes in specified area
// Used for system-wide notifications or state synchronization
// in distributed game server clusters.
// Optimized for fan-out communication patterns with optional self-exclusion.
type RouteTypeBroadCast struct {
	// FuncID identifies the service function for filtering recipients
	FuncID uint32

	// AreaID limits the broadcast scope to specific area
	AreaID uint32

	// ExcludeSelf indicates whether to exclude the sender node from broadcast
	ExcludeSelf bool

	// Precomputed route key for performance optimization
	routeKey string
}

// NewRouteTypeBroadCast creates a new broadcast route with precomputed route key
// Parameters:
// - funcID: The service function identifier
// - areaID: The area identifier to limit broadcast scope
// - excludeSelf: Whether to exclude the sender node from broadcast
// Returns: A new RouteTypeBroadCast instance configured with the specified parameters
func NewRouteTypeBroadCast(funcID uint32, areaID uint32, excludeSelf bool) *RouteTypeBroadCast {
	return &RouteTypeBroadCast{
		FuncID:      funcID,
		AreaID:      areaID,
		ExcludeSelf: excludeSelf,
		routeKey:    fmt.Sprintf("broadcast:%d:%d", funcID, areaID),
	}
}

// GetRouteKey returns the precomputed routing key
// Returns: A string key that uniquely identifies this route configuration
func (r *RouteTypeBroadCast) GetRouteKey() string {
	return r.routeKey
}

// GetRouteStrategy returns the broadcast routing strategy identifier
// Returns: The RouteStrategy constant for broadcast routing
func (r *RouteTypeBroadCast) GetRouteStrategy() RouteStrategy {
	return RouteBroadCast
}

// Validate checks if the function ID is valid
// Returns: nil if valid, error if the function ID is invalid (zero)
func (r *RouteTypeBroadCast) Validate() error {
	if r.FuncID == 0 {
		return fmt.Errorf("invalid func ID: %d", r.FuncID)
	}
	return nil
}

// String returns a human-readable description of the broadcast route
// Returns: Formatted string with routing information for debugging
func (r *RouteTypeBroadCast) String() string {
	return fmt.Sprintf("Broadcast[Func:%d,Area:%d,Exclude:%v]", r.FuncID, r.AreaID, r.ExcludeSelf)
}

// RouteTypeMulCast implements multicast routing to specific entity list
// Used for targeted messaging to selected entities
// in scenarios requiring precise message distribution control.
type RouteTypeMulCast struct {
	// EntityIDs contains the list of target entity identifiers
	EntityIDs []uint64

	// FuncID identifies the service function for routing
	FuncID uint32

	// Precomputed route key for performance optimization
	routeKey string
}

// NewRouteTypeMulCast creates a new multicast route with precomputed route key
// Parameters:
// - funcID: The service function identifier
// - entityIDs: The list of target entity identifiers
// Returns: A new RouteTypeMulCast instance configured with the specified parameters
func NewRouteTypeMulCast(funcID uint32, entityIDs []uint64) *RouteTypeMulCast {
	r := &RouteTypeMulCast{
		FuncID:    funcID,
		EntityIDs: entityIDs,
	}

	if len(entityIDs) == 0 {
		r.routeKey = "multicast:empty"
	} else {
		// Simplified: use first ID as routing key, should compute hash of all IDs in production
		r.routeKey = fmt.Sprintf("multicast:%d:%d", funcID, entityIDs[0])
	}

	return r
}

// GetRouteKey returns the precomputed routing key
// Returns: A string key that uniquely identifies this route configuration
func (r *RouteTypeMulCast) GetRouteKey() string {
	return r.routeKey
}

// GetRouteStrategy returns the multicast routing strategy identifier
// Returns: The RouteStrategy constant for multicast routing
func (r *RouteTypeMulCast) GetRouteStrategy() RouteStrategy {
	return RouteMulCast
}

// Validate checks if the entity list is valid and non-empty
// Returns: nil if valid, error if entity list is empty or contains invalid IDs
func (r *RouteTypeMulCast) Validate() error {
	if len(r.EntityIDs) == 0 {
		return fmt.Errorf("empty entity list")
	}
	for _, id := range r.EntityIDs {
		if id == 0 {
			return fmt.Errorf("invalid entity ID: %d", id)
		}
	}
	return nil
}

// String returns a human-readable description of the multicast route
// Returns: Formatted string with routing information for debugging
func (r *RouteTypeMulCast) String() string {
	return fmt.Sprintf("Multicast[Func:%d,Entities:%d]", r.FuncID, len(r.EntityIDs))
}

// RouteTypeInter implements internal service communication routing
// Used for inter-service RPC calls within the distributed system
// Optimized for service-to-service communication patterns.
type RouteTypeInter struct {
	// ServiceName specifies the target service name for routing
	ServiceName string

	// Method specifies the target method to invoke
	Method string

	// PipeIdx provides the pipeline index for load balancing
	PipeIdx int32

	// Precomputed route key for performance optimization
	routeKey string
}

// NewRouteTypeInter creates a new internal route with precomputed route key
// Parameters:
// - serviceName: The target service name
// - method: The target method to invoke
// - pipeIdx: The pipeline index for load balancing
// Returns: A new RouteTypeInter instance configured with the specified parameters
func NewRouteTypeInter(serviceName string, method string, pipeIdx int32) *RouteTypeInter {
	return &RouteTypeInter{
		ServiceName: serviceName,
		Method:      method,
		PipeIdx:     pipeIdx,
		routeKey:    fmt.Sprintf("inter:%s:%s:%d", serviceName, method, pipeIdx),
	}
}

// GetRouteKey returns the precomputed routing key
// Returns: A string key that uniquely identifies this route configuration
func (r *RouteTypeInter) GetRouteKey() string {
	return r.routeKey
}

// GetRouteStrategy returns the internal communication routing strategy identifier
// Returns: The RouteStrategy constant for internal service communication
func (r *RouteTypeInter) GetRouteStrategy() RouteStrategy {
	return RouteInter
}

// Validate checks if service name and method are valid
// Returns: nil if valid, error if service name or method are empty
func (r *RouteTypeInter) Validate() error {
	if r.ServiceName == "" {
		return fmt.Errorf("empty service name")
	}
	if r.Method == "" {
		return fmt.Errorf("empty method name")
	}
	return nil
}

// String returns a human-readable description of the internal route
// Returns: Formatted string with routing information for debugging
func (r *RouteTypeInter) String() string {
	return fmt.Sprintf("Inter[Service:%s,Method:%s,Pipe:%d]", r.ServiceName, r.Method, r.PipeIdx)
}

// RouteBuilder provides a fluent API for constructing route configurations
// Supports method chaining for convenient route building
// Designed to improve code readability and reduce route configuration errors.
type RouteBuilder struct {
	route Route
}

// NewRouteBuilder creates a new route builder instance for fluent configuration
// Returns: A new RouteBuilder instance ready for method chaining
func NewRouteBuilder() *RouteBuilder {
	return &RouteBuilder{}
}

// P2P configures a point-to-point route targeting specific entity
// Parameters:
// - entityID: The unique identifier of the target entity
// Returns: The builder instance for method chaining
func (b *RouteBuilder) P2P(entityID uint64) *RouteBuilder {
	b.route = NewRouteTypeP2P(entityID)
	return b
}

// Rand configures a random route for load balancing within area
// Parameters:
// - funcID: The service function identifier
// - areaID: The area identifier to limit routing scope
// Returns: The builder instance for method chaining
func (b *RouteBuilder) Rand(funcID uint32, areaID uint32) *RouteBuilder {
	b.route = NewRouteTypeRand(funcID, areaID, false, 0)
	return b
}

// Hash configures a hash-based route for consistent key distribution
// Parameters:
// - funcID: The service function identifier
// - hashKey: The string key for hash calculation
// - areaID: The area identifier to limit routing scope
// Returns: The builder instance for method chaining
func (b *RouteBuilder) Hash(funcID uint32, hashKey string, areaID uint32) *RouteBuilder {
	b.route = NewRouteTypeHash(funcID, hashKey, areaID, 1)
	return b
}

// Build validates and returns the configured route instance
// Returns:
// - The configured Route instance if valid
// - Error if route is not configured or validation fails
func (b *RouteBuilder) Build() (Route, error) {
	if b.route == nil {
		return nil, fmt.Errorf("no route specified")
	}
	if err := b.route.Validate(); err != nil {
		return nil, err
	}
	return b.route, nil
}

// RouteContext provides contextual information for route execution
// Includes tracing and timing information for distributed debugging
// Enhances observability in complex distributed game server environments.
type RouteContext struct {
	context.Context
	Route     Route
	TraceID   string
	StartTime int64
}

// NewRouteContext creates a new route context with tracing information
// Generates unique trace ID for distributed request tracking
// Parameters:
// - ctx: The parent context for cancellation propagation
// - route: The route configuration for message delivery
// Returns: A new RouteContext instance with embedded tracing information
func NewRouteContext(ctx context.Context, route Route) *RouteContext {
	return &RouteContext{
		Context:   ctx,
		Route:     route,
		TraceID:   generateTraceID(),
		StartTime: time.Now().UnixNano(),
	}
}

// generateTraceID generates a unique trace identifier using timestamp
// Returns: A string representation of a unique trace ID
func generateTraceID() string {
	return fmt.Sprintf("trace-%d-%d", time.Now().UnixNano(), rand.Int63())
}
