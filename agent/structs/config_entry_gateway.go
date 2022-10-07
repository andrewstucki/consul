package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// GatewayConfigEntry manages the configuration for an api gateway with the given name.
type GatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.Gateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	Meta map[string]string
	acl.EnterpriseMeta
	RaftIndex
}

func (e *GatewayConfigEntry) GetKind() string {
	return Gateway
}

func (e *GatewayConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *GatewayConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *GatewayConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = Gateway
	e.EnterpriseMeta.Normalize()

	return nil
}

func (e *GatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *GatewayConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *GatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *GatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *GatewayConfigEntry) Validate() error {
	return nil
}

type TCPRouteConfigEntry struct {
	// Kind of the config entry. This will be set to structs.TCPRoute.
	Kind string

	Name string

	Gateways []GatewayReference

	Meta     map[string]string `json:",omitempty"`
	Services []TCPService
	acl.EnterpriseMeta
	RaftIndex
}

type TCPService struct {
	Name   string
	Weight int
	acl.EnterpriseMeta
}

type GatewayReference struct {
	Name string
	acl.EnterpriseMeta
}

func (e *TCPRouteConfigEntry) GetKind() string {
	return TCPRoute
}

func (e *TCPRouteConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *TCPRouteConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *TCPRouteConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = TCPRoute
	e.EnterpriseMeta.Normalize()

	return nil
}

func (e *TCPRouteConfigEntry) Validate() error {
	return nil
}

func (e *TCPRouteConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *TCPRouteConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext)
}

func (e *TCPRouteConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *TCPRouteConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}
