package proxycfg

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/proxycfg/internal/watch"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

type handlerGateway struct {
	handlerState
}

func (s *handlerGateway) initialize(ctx context.Context) (ConfigSnapshot, error) {
	snap := newConfigSnapshotFromServiceInstance(s.serviceInstance, s.stateConfig)
	// Watch for root changes
	err := s.dataSources.CARoots.Notify(ctx, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	// Get information about the entire service mesh.
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.MeshConfig,
		Name:           structs.MeshConfigMesh,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(s.proxyID.PartitionOrDefault()),
	}, meshConfigEntryID, s.ch)
	if err != nil {
		return snap, err
	}

	// Watch this ingress gateway's config entry
	err = s.dataSources.ConfigEntry.Notify(ctx, &structs.ConfigEntryQuery{
		Kind:           structs.Gateway,
		Name:           s.service,
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, gatewayConfigWatchID, s.ch)
	if err != nil {
		return snap, err
	}

	snap.Gateway.TCPRoutes = watch.NewMap[structs.ServiceName, *structs.TCPRouteConfigEntry]()
	snap.Gateway.WatchedDiscoveryChains = make(map[UpstreamID]context.CancelFunc)
	snap.Gateway.DiscoveryChain = make(map[UpstreamID]*structs.CompiledDiscoveryChain)
	snap.Gateway.WatchedUpstreams = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.Gateway.WatchedUpstreamEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.Gateway.WatchedGateways = make(map[UpstreamID]map[string]context.CancelFunc)
	snap.Gateway.WatchedGatewayEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes)
	snap.Gateway.UpstreamPeerTrustBundles = watch.NewMap[string, *pbpeering.PeeringTrustBundle]()
	snap.Gateway.PeerUpstreamEndpoints = watch.NewMap[UpstreamID, structs.CheckServiceNodes]()
	snap.Gateway.PeerUpstreamEndpointsUseHostnames = make(map[UpstreamID]struct{})
	return snap, nil
}

func (s *handlerGateway) handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	switch {
	case u.CorrelationID == rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.Roots = roots
	case u.CorrelationID == gatewayConfigWatchID:
		resp, ok := u.Result.(*structs.ConfigEntryResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		gatewayConf, ok := resp.Entry.(*structs.GatewayConfigEntry)
		if !ok {
			return fmt.Errorf("invalid type for config entry: %T", resp.Entry)
		}

		seenRoutes := make(map[structs.ServiceName]struct{})
		for _, listener := range gatewayConf.Listeners {
			for _, route := range listener.State.TCPRoutes {
				name := structs.NewServiceName(route.Name, &route.EnterpriseMeta)
				seenRoutes[name] = struct{}{}
				{
					childCtx, cancel := context.WithCancel(ctx)
					err := s.dataSources.ConfigEntry.Notify(childCtx, &structs.ConfigEntryQuery{
						Kind:           structs.TCPRoute,
						Name:           route.Name,
						Datacenter:     s.source.Datacenter,
						QueryOptions:   structs.QueryOptions{Token: s.token},
						EnterpriseMeta: route.EnterpriseMeta,
					}, tcpRouteIDPrefix+name.String(), s.ch)
					if err != nil {
						cancel()
						return err
					}
					snap.Gateway.TCPRoutes.InitWatch(name, cancel)
				}
			}	
		}
		
		snap.Gateway.TCPRoutes.ForEachKey(func(name structs.ServiceName) bool {
			if _, ok := seenRoutes[name]; !ok {
				snap.Gateway.TCPRoutes.CancelWatch(name)
			}
			return true
		})

		// do the rest of the gateway struct initialization here

	case strings.HasPrefix(u.CorrelationID, tcpRouteIDPrefix):
	
		// do tcp upstream watch here

	default:
			return fmt.Errorf("watch fired for unsupported kind: %s", snap.Kind)
	}

	return nil
}