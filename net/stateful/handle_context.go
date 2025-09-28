package stateful

import (
	"fmt"

	"github.com/lcx/asura/log"
	"github.com/lcx/asura/net"
	"google.golang.org/protobuf/proto"
)

// beforeSendPkgToClientFunc defines the callback function signature for processing
// packages before they are sent to clients. It allows for custom modifications
// or validations of outgoing client packages.
type beforeSendPkgToClientFunc func(resPkg *net.TransSendPkg) error

// HandleContext manages the execution context for handling incoming messages.
// It provides access to actor information, message data, transport layers,
// and utility functions for sending responses and notifications.
type HandleContext struct {
	log.Logger                                        // Embedded logger for context-aware logging
	*net.DispatcherDelivery                           // Delivery information for the current message
	actorID                 uint64                    // ID of the actor processing this message
	msgMgr                  *net.MessageManager       // Message manager for protocol handling
	msgLayer                *StatefulMsgLayer         // Reference to the stateful message layer
	ntfSender               *net.NtfSender            // Notification sender for client/server communication
	beforeSendPkgToClient   beforeSendPkgToClientFunc // Callback for processing outgoing client packages
}

// newHandleContext creates a new handler context for message processing.
// The logger parameter can be nil and will be set later during context initialization.
// This context is used throughout the message handling pipeline to maintain
// state and provide access to necessary services.
func newHandleContext(aid uint64, dd *net.DispatcherDelivery,
	logger log.Logger) *HandleContext {
	// Logger will be set later during full context initialization
	return &HandleContext{
		actorID:            aid,
		DispatcherDelivery: dd,
		Logger:             logger,
	}
}

// GetActorID returns the ID of the actor associated with this context.
// This is typically the actor that is processing or should process the current message.
func (ctx *HandleContext) GetActorID() uint64 {
	return ctx.actorID
}

// SetActorID manually sets the actor ID for this context.
// This is useful when the actor ID needs to be changed during message processing.
// It also logs the change for debugging purposes.
func (ctx *HandleContext) SetActorID(aid uint64) {
	ctx.actorID = aid
	ctx.Info().Uint64("aid", aid).Msg("manual set actorid")
}

// NtfClient sends a notification package to the client associated with this context.
// It automatically sets the source actor ID and handles any transport-level errors.
// This is commonly used for sending responses or updates back to game clients.
func (ctx *HandleContext) NtfClient(pkg net.TransSendPkg) {
	// The client will also need to handle tail when it is notified.
	pkg.SetSrcActorID(ctx.GetActorID())

	err := ctx.ntfSender.NtfClient(pkg, ctx.actorID, ctx.msgLayer.csTransport)
	if err != nil {
		ctx.Error().Str("msgid", pkg.PkgHdr.GetMsgID()).Err(err).Msg("ntf client")
	}
}

// NtfServer sends a notification package to another server in the cluster.
// This is used for inter-server communication, such as state synchronization
// or migration-related messages between game server instances.
func (ctx *HandleContext) NtfServer(pkg net.TransSendPkg) {
	pkg.SetSrcActorID(ctx.GetActorID())
	if err := ctx.ntfSender.NtfServer(pkg, ctx.msgLayer.ssTransport); err != nil {
		ctx.Error().Err(err).Str("msgid", pkg.PkgHdr.GetMsgID()).Msg("ntf server fail")
	}
}

// sendBack sends a response message back to the original sender.
// It creates a response package with the provided error code and body,
// then sends it using the transport's sendback mechanism.
// A nil body is valid for responses that only need to indicate success/failure.
func (ctx *HandleContext) sendBack(code int32, body proto.Message) (err error) {
	if ctx.TransSendBack == nil {
		return fmt.Errorf("MsgID:%s no sendback function", ctx.Pkg.PkgHdr.GetMsgID())
	}

	resPkg, err := net.NewResPkg(ctx.Pkg, ctx.ProtoInfo.GetResMsgID(), code, body, ctx.ResOpts...)
	if err != nil {
		return err
	}
	err = ctx.TransSendBack(resPkg)
	return
}
