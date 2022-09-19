package xds

import (
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_router_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/golang/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
)

func (s *ResourceGenerator) makeGatewayListeners(address string, cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	s.Logger.Debug("handling gateway listener xDS")

	router, err := makeEnvoyHTTPFilter("envoy.filters.http.router", &envoy_http_router_v3.Router{})
	if err != nil {
		return nil, err
	}

	manager := &envoy_http_v3.HttpConnectionManager{
		StatPrefix: "upstream_stubs_",
		CodecType:  envoy_http_v3.HttpConnectionManager_AUTO,
		HttpFilters: []*envoy_http_v3.HttpFilter{
			router,
		},
		RouteSpecifier: &envoy_http_v3.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy_route_v3.RouteConfiguration{
				Name: "stubbed-route",
				VirtualHosts: []*envoy_route_v3.VirtualHost{
					{
						Name:    "stubbed-filter",
						Domains: []string{"*"},
						Routes: []*envoy_route_v3.Route{
							&envoy_route_v3.Route{
								Match: &envoy_route_v3.RouteMatch{
									PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{
										Prefix: "/",
									},
								},
								Action: &envoy_route_v3.Route_DirectResponse{
									DirectResponse: &envoy_route_v3.DirectResponseAction{
										Status: 200,
										Body: &envoy_config_core_v3.DataSource{
											Specifier: &envoy_config_core_v3.DataSource_InlineString{
												InlineString: "stub",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		Tracing: &envoy_http_v3.HttpConnectionManager_Tracing{
			RandomSampling: &envoy_type_v3.Percent{Value: 0.0},
		},
	}

	filter, err := makeFilter("envoy.filters.network.http_connection_manager", manager)
	if err != nil {
		return nil, err
	}

	listener := &envoy_listener_v3.Listener{
		Name:       "stubbed-listener",
		StatPrefix: "gateway_upstream_",
		Address: &envoy_config_core_v3.Address{
			Address: &envoy_config_core_v3.Address_SocketAddress{
				SocketAddress: &envoy_config_core_v3.SocketAddress{
					Address: "::",
					PortSpecifier: &envoy_config_core_v3.SocketAddress_PortValue{
						PortValue: 9330,
					},
				},
			},
		},
		TrafficDirection: envoy_config_core_v3.TrafficDirection_OUTBOUND,
		FilterChains: []*envoy_listener_v3.FilterChain{{
			Filters: []*envoy_listener_v3.Filter{
				filter,
			},
		}},
	}

	return []proto.Message{listener}, nil
}
