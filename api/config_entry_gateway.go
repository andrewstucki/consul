package api

import (
	"github.com/hashicorp/consul/types"
)

type AsyncStatus struct {
	Conditions []Condition `json:",omitempty"`
}

type Condition struct {
	// status of the condition, one of True, False, Unknown.
	Status string
	// observedGeneration represents the .metadata.generation that the condition was set based upon.
	// For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
	// with respect to the current state of the instance.
	ObservedGeneration int64
	// lastTransitionTime is the last time the condition transitioned from one status to another.
	// This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
	LastTransitionTime uint64
	// reason contains a programmatic identifier indicating the reason for the condition's last transition.
	// Producers of specific condition types may define expected values and meanings for this field,
	// and whether the values are considered a guaranteed API.
	// The value should be a CamelCase string.
	// This field may not be empty.
	Reason string
	// message is a human readable message indicating details about the transition.
	// This may be an empty string.
	Message string
}

// GatewayConfigEntry manages the configuration for an api gateway with the given name.
type GatewayConfigEntry struct {
	// Kind of the config entry. This will be set to structs.Gateway.
	Kind string

	// Name is used to match the config entry with its associated ingress gateway
	// service. This should match the name provided in the service definition.
	Name string

	// Partition is the partition the IngressGateway is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the IngressGateway is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	// Listeners declares what ports the ingress gateway should listen on, and
	// what services to associated to those ports.
	Listeners []GatewayListener

	Meta map[string]string `json:",omitempty"`

	Status AsyncStatus

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64
}

func (g *GatewayConfigEntry) GetKind() string            { return g.Kind }
func (g *GatewayConfigEntry) GetName() string            { return g.Name }
func (g *GatewayConfigEntry) GetPartition() string       { return g.Partition }
func (g *GatewayConfigEntry) GetNamespace() string       { return g.Namespace }
func (g *GatewayConfigEntry) GetMeta() map[string]string { return g.Meta }
func (g *GatewayConfigEntry) GetCreateIndex() uint64     { return g.CreateIndex }
func (g *GatewayConfigEntry) GetModifyIndex() uint64     { return g.ModifyIndex }

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
	Name string
	// Partition is the partition the IngressGateway is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the IngressGateway is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`
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

// TCPRouteConfigEntry manages the configuration for a TCP gateway route.
type TCPRouteConfigEntry struct {
	// Kind of the config entry. This will be set to structs.TCPRoute.
	Kind string

	Name string

	Gateways []GatewayReference

	Meta     map[string]string `json:",omitempty"`
	Status   AsyncStatus
	Services []TCPService

	// Partition is the partition the IngressGateway is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the IngressGateway is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`

	// CreateIndex is the Raft index this entry was created at. This is a
	// read-only field.
	CreateIndex uint64

	// ModifyIndex is used for the Check-And-Set operations and can also be fed
	// back into the WaitIndex of the QueryOptions in order to perform blocking
	// queries.
	ModifyIndex uint64
}

func (g *TCPRouteConfigEntry) GetKind() string            { return g.Kind }
func (g *TCPRouteConfigEntry) GetName() string            { return g.Name }
func (g *TCPRouteConfigEntry) GetPartition() string       { return g.Partition }
func (g *TCPRouteConfigEntry) GetNamespace() string       { return g.Namespace }
func (g *TCPRouteConfigEntry) GetMeta() map[string]string { return g.Meta }
func (g *TCPRouteConfigEntry) GetCreateIndex() uint64     { return g.CreateIndex }
func (g *TCPRouteConfigEntry) GetModifyIndex() uint64     { return g.ModifyIndex }

type TCPService struct {
	Name   string
	Weight int
	// Partition is the partition the IngressGateway is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the IngressGateway is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`
}

type GatewayReference struct {
	Name string
	// Partition is the partition the IngressGateway is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the IngressGateway is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`
}
