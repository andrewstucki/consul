package controllers

import (
	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
)

// TCPRouteController creates a new Controller with a TCPRouteReconciler
func TCPRouteController(fsm *fsm.FSM, updater *Updater, publisher state.EventPublisher, logger hclog.Logger) controller.Controller {
	reconciler := NewRouteReconciler(logger, fsm, updater, SynthesizeTCPRouteChain)

	controller := controller.New(publisher, reconciler)
	reconciler.controller = controller

	return controller.Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicTCPRoute,
		Subject: stream.SubjectWildcard,
	})
}

func SynthesizeTCPRouteChain(store *state.Store, route *structs.TCPRouteConfigEntry) (memdb.WatchSet, *configentry.DiscoveryChainSet, error) {
	ws := memdb.NewWatchSet()
	ws.Add(store.AbandonCh())

	for _, service := range route.Services {
		_, chains, err := store.ReadDiscoveryChainConfigEntries(ws, service.Name, nil, &service.EnterpriseMeta)
		if err != nil {
			return nil, nil, err
		}
		// return after the first entry, we should only have one for now
		return ws, chains, nil
	}

	return nil, nil, nil
}
