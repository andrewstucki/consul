package controllers

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
)

// TCPRouteReconciler is a reconciliation control loop
// handler for TCP routes
type TCPRouteReconciler struct {
	logger     hclog.Logger
	fsm        *fsm.FSM
	controller controller.Controller
}

// Reconcile reconciles TCPRoute config entries.
func (r *TCPRouteReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	store := r.fsm.State()

	idx, entry, err := store.ConfigEntry(nil, req.Kind, req.Name, &req.Meta)
	if err != nil {
		return err
	}
	if entry == nil {
		r.logger.Error("cleaning up deleted TCP object", "request", req)
		r.controller.RemoveTrigger(req)
		return nil
	}
	r.logger.Error("got tcp reconcile call", "request", req)

	route := entry.(*structs.TCPRouteConfigEntry)

	ws := memdb.NewWatchSet()
	ws.Add(store.AbandonCh())
	for _, service := range route.Services {
		_, chains, err := store.ReadDiscoveryChainConfigEntries(ws, service.Name, nil, &service.EnterpriseMeta)
		if err != nil {
			return err
		}
		r.logger.Trace("fetched chains", "chains", chains)
	}
	r.controller.AddTrigger(req, ws.WatchCtx)

	r.logger.Trace("compare and swap TODO", "idx", idx)

	return nil
}

// TCPRouteController creates a new Controller with a TCPRouteReconciler
func TCPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := &TCPRouteReconciler{
		fsm:    fsm,
		logger: logger,
	}
	controller := controller.New(publisher, reconciler)
	reconciler.controller = controller
	return controller.Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicTCPRoute,
		Subject: stream.SubjectWildcard,
	})
}
