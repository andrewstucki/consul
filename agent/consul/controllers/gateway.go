package controllers

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/go-hclog"
)

// GatewayReconciler is a reconciliation control loop
// handler for gateways
type GatewayReconciler struct {
	logger hclog.Logger
	fsm    *fsm.FSM
}

// GatewayReconciler reconciles Gateway config entries.
func (r *GatewayReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	r.logger.Error("got gateay reconcile call", "request", req)
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
