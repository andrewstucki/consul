package api

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

	Meta map[string]string `json:",omitempty"`

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

// TCPRouteConfigEntry manages the configuration for a TCP gateway route.
type TCPRouteConfigEntry struct {
	// Kind of the config entry. This will be set to structs.TCPRoute.
	Kind string

	Name string

	Parents []ParentReference

	Meta     map[string]string `json:",omitempty"`
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

type ParentReference struct {
	Kind        string
	Name        string
	SectionName string
	// Partition is the partition the IngressGateway is associated with.
	// Partitioning is a Consul Enterprise feature.
	Partition string `json:",omitempty"`

	// Namespace is the namespace the IngressGateway is associated with.
	// Namespacing is a Consul Enterprise feature.
	Namespace string `json:",omitempty"`
}
