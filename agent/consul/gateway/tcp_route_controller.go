package gateway

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
)

// TCPRouteReconciler is a reconciliation control loop
// handler for TCP routes
type TCPRouteReconciler struct {
	logger hclog.Logger
	store  controller.Store
}

// Reconcile reconciles TCPRoute config entries.
func (r *TCPRouteReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	r.logger.Error("got tcp reconcile call", "request", req)
	return nil
}

// TCPRouteController creates a new Controller with a TCPRouteReconciler
func TCPRouteController(store controller.Store, logger hclog.Logger) controller.Controller {
	return controller.New(store, &TCPRouteReconciler{
		store:  store,
		logger: logger,
	}).Watch(structs.TCPRoute, nil)
}
