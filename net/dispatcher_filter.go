// Package net provides network communication components for the Asura framework,
// including message routing, dispatching, and transport mechanisms.
package net

import (
	"errors"
)

// DispatcherFilterHandleFunc defines the function signature for filter chain handlers.
// These functions are responsible for processing messages in the filter chain,
// typically performing validation, logging, or transformation before allowing
// the message to proceed to the next handler.
//
// Parameters:
// - dd: A pointer to the DispatcherDelivery containing the message to process
//
// Returns:
// An error if processing fails, or nil if successful

type DispatcherFilterHandleFunc func(dd *DispatcherDelivery) error

// DispatcherFilter defines a filter (interceptor) function that can be inserted
// into the dispatcher processing pipeline. Filters allow for cross-cutting concerns
// like authentication, logging, rate limiting, or message transformation to be
// applied consistently across all message processing.
//
// Parameters:
// - dd: A pointer to the DispatcherDelivery containing the message to filter
// - f: The next filter handle function in the chain to call after processing
//
// Returns:
// An error if filtering fails, or the result of the next filter in the chain

type DispatcherFilter func(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error

// DispatcherFilterChain represents a chain of filters that process messages
// sequentially in a pipeline pattern. Each filter can decide whether to continue
// processing the message through the chain or terminate the chain early.
//
// This implementation uses recursion to process the filter chain, with each
// filter calling the next one in the chain after performing its own processing.

type DispatcherFilterChain []DispatcherFilter

// Handle processes a message through the entire filter chain using recursion.
// If the chain is empty, it directly calls the provided final handler function.
// Otherwise, it calls the first filter in the chain and passes a closure that
// processes the remaining filters recursively.
//
// Parameters:
// - dd: A pointer to the DispatcherDelivery containing the message to process
// - f: The final handler function to call after all filters have processed the message
//
// Returns:
// An error if any filter or the final handler fails, or nil if processing succeeds

func (fc DispatcherFilterChain) Handle(dd *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
	if len(fc) == 0 {
		return f(dd)
	}
	return fc[0](dd, func(dd *DispatcherDelivery) error {
		return fc[1:].Handle(dd, f)
	})
}

// reloadMsgFilterCfg reloads the message filter configuration when it changes.
// It updates the dispatcher's message filter map with new message IDs to filter.
//
// Parameters:
// - cfg: A pointer to the new message filter configuration
//
// This method is typically called during application initialization or when
// configuration changes are detected at runtime.

func (dp *Dispatcher) reloadMsgFilterCfg(cfg *MsgFilterPluginCfg) {
	for _, msgName := range cfg.MsgFilter {
		dp.msgFilterMap[msgName] = struct{}{}
	}
}

// msgFilter implements a message filtering mechanism for the dispatcher.
// It checks if a message ID is in the filter map and, if so, handles it
// by sending back an empty response without passing it to the next handler.
//
// Parameters:
// - d: A pointer to the DispatcherDelivery containing the message to filter
// - f: The next filter handle function in the chain
//
// Returns:
// An error if filtering fails, nil if the message was filtered successfully,
// or the result of the next filter if the message should not be filtered
//
// This filter is used to implement message blocking or quick response generation
// for specific message types, typically those that don't require full processing.

func (dp *Dispatcher) msgFilter(d *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
	if d.Pkg.PkgHdr == nil {
		return errors.New("dd Header is nil")
	}
	hdr := d.Pkg.PkgHdr
	if _, ok := dp.msgFilterMap[hdr.GetMsgID()]; !ok {
		return f(d)
	}
	pi, ok := dp.msgMgr.GetProtoInfo(hdr.GetMsgID())
	if !ok {
		return errors.New("GetProtoInfo Failed")
	}
	if !pi.IsReq() {
		return nil
	}
	resPkg := &TransSendPkg{
		PkgHdr: &PkgHead{
			MsgID:      pi.ResMsgID,
			DstActorID: hdr.GetSrcActorID(),
			SrcActorID: hdr.GetDstActorID(),
			SvrPkgSeq:  hdr.GetSvrPkgSeq(),
			CltPkgSeq:  hdr.GetCltPkgSeq(),
		},
	}

	if d.TransSendBack != nil {
		return d.TransSendBack(resPkg)
	}
	return nil
}
