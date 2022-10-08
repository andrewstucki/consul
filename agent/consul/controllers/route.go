package controllers

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
)

type Updater struct {
	UpdateStatus func(entry structs.ConfigEntry) error
	Update       func(entry structs.ConfigEntry) error
	Delete       func(entry structs.ConfigEntry) error
}

// RouteReconciler is a reconciliation control loop
// handler for routes
type RouteReconciler[T structs.Route] struct {
	logger      hclog.Logger
	fsm         *fsm.FSM
	controller  controller.Controller
	updater     *Updater
	synthesizer func(store *state.Store, route T) (memdb.WatchSet, *configentry.DiscoveryChainSet, error)
}

func NewRouteReconciler[T structs.Route](
	logger hclog.Logger,
	fsm *fsm.FSM,
	updater *Updater,
	synthesizer func(store *state.Store, route T) (memdb.WatchSet, *configentry.DiscoveryChainSet, error),
) *RouteReconciler[T] {
	reconciler := new(RouteReconciler[T])
	reconciler.logger = logger
	reconciler.fsm = fsm
	reconciler.updater = updater
	reconciler.synthesizer = synthesizer
	return reconciler
}

// Reconcile reconciles Route config entries.
func (r *RouteReconciler[T]) Reconcile(ctx context.Context, req controller.Request) error {
	store := r.fsm.State()

	_, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, &req.Meta)
	if err != nil {
		return err
	}
	if entry == nil {
		r.logger.Error("cleaning up deleted route object", "request", req)
		toUpdate, err := RemoveRoute(store, ConstructBoundRouteFromRequest(req))
		if err != nil {
			return err
		}
		for _, state := range toUpdate {
			if err := r.updater.Update(state); err != nil {
				return err
			}
		}
		r.controller.RemoveTrigger(req)
		return nil
	}
	r.logger.Error("got route reconcile call", "request", req)

	route := entry.(T)
	ws, chain, err := r.synthesizer(store, route)
	if err != nil {
		return err
	}
	toUpdate, bindErrors, err := BindGateways(store, ConstructBoundRoute(route, chain))
	if err != nil {
		return err
	}
	for reference, err := range bindErrors {
		// do something with the status here
		r.logger.Trace("foo", "ref", reference, "error", err)
	}

	// first update all of the state values
	for _, state := range toUpdate {
		r.logger.Debug("persisting gateway state", "state", state)
		if err := r.updater.Update(state); err != nil {
			r.logger.Error("error persisting state", "error", err)
			return err
		}
	}

	// now update the route status
	if err := r.updater.UpdateStatus(route); err != nil {
		return err
	}

	if ws != nil {
		// finally add the trigger
		r.controller.AddTrigger(req, ws.WatchCtx)
	}

	return nil
}

func ConstructBoundRouteFromRequest(request controller.Request) structs.BoundRoute {
	return structs.BoundRoute{
		Kind:           request.Kind,
		Name:           request.Name,
		EnterpriseMeta: request.Meta,
	}
}

func ConstructBoundRoute(route structs.Route, chain *configentry.DiscoveryChainSet) structs.BoundRoute {
	meta := route.GetEnterpriseMeta()
	if meta == nil {
		meta = acl.DefaultEnterpriseMeta()
	}
	bound := structs.BoundRoute{
		Kind:           route.GetKind(),
		Name:           route.GetName(),
		EnterpriseMeta: *meta,
		Parents:        route.GetParents(),
	}
	if chain != nil {
		routers, splitters, resolvers, defaults, proxies := chain.Flatten()
		bound.Routers = routers
		bound.Splitters = splitters
		bound.Resolvers = resolvers
		bound.Services = defaults
		bound.ProxyDefaults = proxies
	}
	return bound
}

func RemoveRoute(store *state.Store, route structs.BoundRoute) ([]*structs.BoundGatewayConfigEntry, error) {
	modified := []*structs.BoundGatewayConfigEntry{}

	_, entries, err := store.ConfigEntriesByKind(nil, structs.BoundGateway, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		state := entry.(*structs.BoundGatewayConfigEntry)
		if state.RemoveRoute(route) {
			modified = append(modified, state)
		}
	}

	return modified, nil
}

func BindGateways(store *state.Store, route structs.BoundRoute) ([]*structs.BoundGatewayConfigEntry, map[structs.ParentReference]error, error) {
	referenceSet := make(map[structs.ParentReference]struct{})
	for _, reference := range route.Parents {
		referenceSet[reference] = struct{}{}
	}

	modifiedState := make(map[configentry.KindName]*structs.BoundGatewayConfigEntry)
	errored := make(map[structs.ParentReference]error)

	_, entries, err := store.ConfigEntriesByKind(nil, structs.BoundGateway, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, nil, err
	}

	for _, entry := range entries {
		state := entry.(*structs.BoundGatewayConfigEntry)
		key := configentry.NewKindNameForEntry(state)
		// actually filter
		for reference := range referenceSet {
			bound, err := state.BindRoute(reference, route)
			if err != nil {
				// we have an invalid reference or failed binding
				// validation, track the error
				errored[reference] = err
				delete(referenceSet, reference)
				continue
			}
			if bound {
				delete(referenceSet, reference)
				modifiedState[key] = state
			}
		}
		// now we clean up the state if it has a stale reference
		if _, ok := modifiedState[key]; ok && state.RemoveRoute(route) {
			modifiedState[key] = state
		}
	}

	// add all the references that aren't bound at this point to the error set
	for reference := range referenceSet {
		errored[reference] = errors.New("invalid reference")
	}

	modified := []*structs.BoundGatewayConfigEntry{}
	for _, state := range modifiedState {
		modified = append(modified, state)
	}

	return modified, errored, nil
}
