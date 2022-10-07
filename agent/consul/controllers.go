package consul

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controllers"
	"github.com/hashicorp/consul/logging"
	"golang.org/x/sync/errgroup"
)

func (s *Server) runControllers(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return controllers.GatewayController(s.FSM(), s.publisher, s.logger.Named(logging.GatewayController)).Start(groupCtx)
	})
	group.Go(func() error {
		return controllers.TCPRouteController(s.FSM(), s.publisher, s.logger.Named(logging.TCPRouteController)).Start(groupCtx)
	})

	return group.Wait()
}
