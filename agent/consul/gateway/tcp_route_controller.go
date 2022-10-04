package gateway

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-hclog"
)

// TCPRouteReconciler is a reconciliation control loop
// handler for TCP routes
type TCPRouteReconciler struct {
	logger hclog.Logger
	fsm    *fsm.FSM
}

// Reconcile reconciles TCPRoute config entries.
func (r *TCPRouteReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	r.logger.Error("got tcp reconcile call", "request", req)
	return nil
}

// TCPRouteController creates a new Controller with a TCPRouteReconciler
func TCPRouteController(fsm *fsm.FSM, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	return controller.New(publisher, &TCPRouteReconciler{
		fsm:    fsm,
		logger: logger,
	}).Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicTCPRoute,
		Subject: stream.SubjectWildcard,
	})
}
