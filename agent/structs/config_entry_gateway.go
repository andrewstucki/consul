package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"golang.org/x/exp/slices"
)

type BoundRoute struct {
	Kind string
	Name string
	acl.EnterpriseMeta

	Parents []ParentReference

	Routers       []*ServiceRouterConfigEntry
	Splitters     []*ServiceSplitterConfigEntry
	Resolvers     []*ServiceResolverConfigEntry
	Services      []*ServiceConfigEntry
	ProxyDefaults []*ProxyConfigEntry
}

func (r BoundRoute) EqualReferences(other BoundRoute) bool {
	return r.Kind == other.Kind && r.Name == other.Name && r.EnterpriseMeta.IsSame(&other.EnterpriseMeta)
}

type Route interface {
	ConfigEntry
	GetParents() []ParentReference
}

type BoundListener struct {
	Name   string
	Routes []BoundRoute
}

func (e *BoundListener) BindRoute(route BoundRoute) (bool, error) {
	// do the actual validation logic for binding here
	// as a stub we always bind
	for i, r := range e.Routes {
		if r.EqualReferences(route) {
			// update in place
			e.Routes[i] = route
			return true, nil
		}
	}
	e.Routes = append(e.Routes, route)
	return true, nil
}

func (e *BoundListener) RemoveRoute(route BoundRoute) bool {
	for i, r := range e.Routes {
		if r.EqualReferences(route) {
			e.Routes = slices.Delete(e.Routes, i, i+1)
			return true
		}
	}
	return false
}

// BoundGatewayConfigEntry manages the configuration for an api gateway with the given name.
type BoundGatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.Gateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	Listeners []BoundListener

	Meta map[string]string
	acl.EnterpriseMeta
	RaftIndex
}

func (e *BoundGatewayConfigEntry) GetKind() string {
	return BoundGateway
}

func (e *BoundGatewayConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *BoundGatewayConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *BoundGatewayConfigEntry) Normalize() error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	e.Kind = BoundGateway
	e.EnterpriseMeta.Normalize()

	return nil
}

func (e *BoundGatewayConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().MeshReadAllowed(&authzContext)
}

func (e *BoundGatewayConfigEntry) CanWrite(authz acl.Authorizer) error {
	return acl.PermissionDeniedByACLUnnamed(authz.ToAllowAuthorizer(), nil, acl.Resource("internal"), acl.AccessWrite)
}

func (e *BoundGatewayConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *BoundGatewayConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *BoundGatewayConfigEntry) Validate() error {
	return nil
}

func (e *BoundGatewayConfigEntry) BindRoute(reference ParentReference, route BoundRoute) (bool, error) {
	if reference.Kind != Gateway || reference.Name != e.Name || !reference.EnterpriseMeta.IsSame(&e.EnterpriseMeta) {
		return false, nil
	}

	for _, r := range e.Listeners {
		if r.Name == reference.SectionName {
			return r.BindRoute(route)
		}
	}

	return false, fmt.Errorf("invalid section name: %s", reference.SectionName)
}

func (e *BoundGatewayConfigEntry) RemoveRoute(route BoundRoute) bool {
	for _, r := range e.Listeners {
		if r.RemoveRoute(route) {
			return true
		}
	}
	return false
}

// GatewayConfigEntry manages the configuration for an api gateway with the given name.
type GatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.Gateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	Listeners []GatewayListener

	Meta map[string]string
	acl.EnterpriseMeta
	RaftIndex
}

type GatewayListener struct {
	Name string
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

	Parents []ParentReference

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

type ParentReference struct {
	Kind        string
	Name        string
	SectionName string
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

func (e *TCPRouteConfigEntry) GetParents() []ParentReference {
	return e.Parents
}
