package structs

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/types"
)

// GatewayConfigEntry manages the configuration for an api gateway with the given name.
type GatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.Gateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	// Listeners declares what ports the ingress gateway should listen on, and
	// what services to associated to those ports.
	Listeners []GatewayListener

	Meta               map[string]string `json:",omitempty"`
	Status             Status
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

type GatewayListener struct {
	// Port declares the port on which the gateway should listen for traffic.
	Port int

	// Name declares the name of the listener for binding consideration by
	// routes.
	Name string `json:",omitempty"`

	// Protocol declares what type of traffic this listener is expected to
	// receive. Depending on the protocol, a listener might support multiplexing
	// services over a single port, or additional discovery chain features. The
	// current supported values are: (tcp | http).
	Protocol string

	// TLS config for this listener.
	TLS *GatewayTLSConfiguration `json:",omitempty"`

	State GatewayListenerState
}

type GatewayListenerState struct {
	HTTPRoutes []RouteReference `json:",omitempty"`
	TCPRoutes  []RouteReference `json:",omitempty"`
}

type RouteReference struct {
	Name               string
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

type GatewayTLSConfiguration struct {
	Certificates []GatewayTLSCertificate `json:",omitempty"`

	MaxVersion types.TLSVersion `json:",omitempty" alias:"tls_min_version"`
	MinVersion types.TLSVersion `json:",omitempty" alias:"tls_max_version"`

	// Define a subset of cipher suites to restrict
	// Only applicable to connections negotiated via TLS 1.2 or earlier
	CipherSuites []types.TLSCipherSuite `json:",omitempty" alias:"cipher_suites"`
}

type GatewayTLSCertificate struct {
	VaultKV *GatewayVaultKVCertificate `json:",omitempty" alias:"vault_kv"`
}

type GatewayVaultKVCertificate struct {
	Path            string
	ChainField      string `json:",omitempty" alias:"certificate_chain_field"`
	PrivateKeyField string `json:",omitempty" alias:"private_key_field"`
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

func validateListenerTLS(tlsCfg GatewayTLSConfiguration) error {
	return validateTLSConfig(tlsCfg.MinVersion, tlsCfg.MaxVersion, tlsCfg.CipherSuites)
}

func (e *GatewayConfigEntry) Validate() error {
	if err := validateConfigEntryMeta(e.Meta); err != nil {
		return err
	}

	validProtocols := map[string]bool{
		"tcp":  true,
		"http": true,
	}
	declaredPorts := make(map[int]bool)

	for _, listener := range e.Listeners {
		if _, ok := declaredPorts[listener.Port]; ok {
			return fmt.Errorf("port %d declared on two listeners", listener.Port)
		}
		declaredPorts[listener.Port] = true

		if _, ok := validProtocols[listener.Protocol]; !ok {
			return fmt.Errorf("protocol must be 'tcp', 'http', 'http2', or 'grpc'. '%s' is an unsupported protocol", listener.Protocol)
		}

		if listener.TLS != nil {
			if err := validateListenerTLS(*listener.TLS); err != nil {
				return err
			}
		}
	}

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

func (e *GatewayConfigEntry) ShouldUpdate(existing ControlledConfigEntry) bool {
	return true
}

// TCPRouteConfigEntry manages the configuration for a TCP gateway route.
type TCPRouteConfigEntry struct {
	// Kind of the config entry. This will be set to structs.TCPRoute.
	Kind string

	Name string

	Gateways []GatewayReference

	Meta               map[string]string `json:",omitempty"`
	Status             Status
	Services           []TCPService
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}

type TCPService struct {
	Name               string
	Weight             int
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
}

type GatewayReference struct {
	Name               string
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
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

func (e *TCPRouteConfigEntry) ShouldUpdate(existing ControlledConfigEntry) bool {
	return true
}
