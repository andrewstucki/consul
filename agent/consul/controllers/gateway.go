package controllers

import (
	"context"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
)

// GatewayReconciler is a reconciliation control loop
// handler for gateways
type GatewayReconciler struct {
	logger  hclog.Logger
	fsm     *fsm.FSM
	updater *Updater
}

// GatewayReconciler reconciles Gateway config entries.
func (r *GatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	store := r.fsm.State()

	_, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, &req.Meta)
	if err != nil {
		return err
	}

	if entry == nil {
		r.logger.Error("cleaning up deleted gateway object", "request", req)
		if err := r.updater.Delete(&structs.BoundGatewayConfigEntry{
			Kind:           structs.BoundGateway,
			Name:           req.Name,
			EnterpriseMeta: req.Meta,
		}); err != nil {
			return err
		}
		return nil
	}

	gateway := entry.(*structs.GatewayConfigEntry)
	// TODO: do initial distributed validation here, if we're invalid, then set the status

	var state *structs.BoundGatewayConfigEntry
	_, boundEntry, err := store.ConfigEntry(nil, structs.BoundGateway, req.Name, &req.Meta)
	if err != nil {
		return err
	}
	if boundEntry == nil {
		state = &structs.BoundGatewayConfigEntry{
			Kind:           structs.BoundGateway,
			Name:           gateway.Name,
			EnterpriseMeta: gateway.EnterpriseMeta,
		}
	} else {
		state = boundEntry.(*structs.BoundGatewayConfigEntry)
	}

	r.logger.Debug("started reconciling gateway")
	routes, err := BindTCPRoutes(store, state)
	if err != nil {
		return err
	}

	// now update the gateway state
	r.logger.Debug("persisting gateway state", "state", state)
	if err := r.updater.Update(state); err != nil {
		r.logger.Error("error persisting state", "error", err)
		return err
	}

	// then update the gateway status
	r.logger.Debug("persisting gateway status", "gateway", gateway)
	if err := r.updater.UpdateStatus(gateway); err != nil {
		return err
	}

	// and update the route statuses
	for _, route := range routes {
		r.logger.Debug("persisting route status", "route", route)
		if err := r.updater.UpdateStatus(route); err != nil {
			return err
		}
	}

	r.logger.Debug("finished reconciling gateway")
	return nil
}

// GatewayController creates a new Controller with a TCPRouteReconciler
func GatewayController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	return controller.New(publisher, &GatewayReconciler{
		fsm:    fsm,
		logger: logger,
	}).Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicGateway,
		Subject: stream.SubjectWildcard,
	})
}

func BindTCPRoutes(store *state.Store, state *structs.BoundGatewayConfigEntry) ([]*structs.TCPRouteConfigEntry, error) {
	return bindRoutes(store, structs.TCPRoute, SynthesizeTCPRouteChain, state)
}

func bindRoutes[T structs.Route](store *state.Store, kind string, synthesizer func(store *state.Store, route T) (memdb.WatchSet, *configentry.DiscoveryChainSet, error), state *structs.BoundGatewayConfigEntry) ([]T, error) {
	_, entries, err := store.ConfigEntriesByKind(nil, kind, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, err
	}

	modifiedRoutes := make(map[configentry.KindName]T)

	for _, entry := range entries {
		route := entry.(T)
		key := configentry.NewKindNameForEntry(route)

		_, chain, err := synthesizer(store, route)
		if err != nil {
			return nil, err
		}
		bindable := ConstructBoundRoute(route, chain)

		for _, reference := range bindable.Parents {
			bound, err := state.BindRoute(reference, bindable)
			if err != nil {
				// TODO: we have an invalid reference or failed binding
				// validation, track the error by adding it to the route status
				modifiedRoutes[key] = route
				break
			}
			if bound {
				// TODO: set the validation status here
				modifiedRoutes[key] = route
			}
		}
	}

	modified := []T{}
	for _, route := range modifiedRoutes {
		modified = append(modified, route)
	}
	return modified, nil
}
