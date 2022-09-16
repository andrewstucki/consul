package gateway

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
)

// GatewayReconciler is a reconciliation control loop
// handler for API Gateway
type GatewayReconciler struct {
	logger hclog.Logger
	store  controller.Store
}

// Reconcile reconciles Gateway config entries.
func (r *GatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	r.logger.Error("got gateway reconcile call", "request", req)
	return nil
}

// GatewayController creates a new Controller with a GatewayReconciler
func GatewayController(store controller.Store, logger hclog.Logger) controller.Controller {
	return controller.New(store, &GatewayReconciler{
		store:  store,
		logger: logger,
	}).Watch(structs.Gateway, nil)
}
