package consul

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/gateway"
	"github.com/hashicorp/consul/logging"
	"golang.org/x/sync/errgroup"
)

func (s *Server) runControllers(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)

	store := s.FSM().State()
	group.Go(func() error {
		return gateway.Register(store, s.logger.Named(logging.GatewayController)).Start(groupCtx)
	})

	return group.Wait()
}
