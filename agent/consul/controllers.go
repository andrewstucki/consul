package consul

import (
	"context"

	"github.com/hashicorp/consul/agent/consul/controllers"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"golang.org/x/sync/errgroup"
)

func (s *Server) runControllers(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)

	updater := &controllers.Updater{
		UpdateStatus: func(entry structs.ConfigEntry) error {
			return nil
		},
		Update: func(entry structs.ConfigEntry) error {
			_, err := s.leaderRaftApply("ConfigEntry.Apply", structs.ConfigEntryRequestType, &structs.ConfigEntryRequest{
				Op:    structs.ConfigEntryUpsertCAS,
				Entry: entry,
			})
			return err
		},
		Delete: func(entry structs.ConfigEntry) error {
			_, err := s.leaderRaftApply("ConfigEntry.Delete", structs.ConfigEntryRequestType, &structs.ConfigEntryRequest{
				Op:    structs.ConfigEntryDelete,
				Entry: entry,
			})
			return err
		},
	}

	group.Go(func() error {
		logger := s.logger.Named(logging.GatewayController)
		return controllers.GatewayController(s.FSM(), s.publisher, logger).WithLogger(logger).Start(groupCtx)
	})
	group.Go(func() error {
		logger := s.logger.Named(logging.TCPRouteController)
		return controllers.TCPRouteController(s.FSM(), updater, s.publisher, logger).WithLogger(logger).Start(groupCtx)
	})

	return group.Wait()
}
