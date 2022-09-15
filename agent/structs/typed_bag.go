package structs

import (
	"github.com/hashicorp/consul/acl"
)

type TypedBagOp string

const (
	TypedBagUpsert TypedBagOp = "upsert"
	TypedBagDelete TypedBagOp = "delete"
)

type TypedBagRequest struct {
	Op         TypedBagOp
	Datacenter string
	Bag        TypedBag

	WriteRequest
}

func (r *TypedBagRequest) RequestDatacenter() string {
	return r.Datacenter
}

type TypedBagQuery struct {
	Datacenter  string
	KindVersion KindVersion
	Name        string

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	QueryOptions
}

func (r *TypedBagQuery) RequestDatacenter() string {
	return r.Datacenter
}

type TypedBagResponse struct {
	TypedBag *TypedBag
	QueryMeta
}

type TypedBagListResponse struct {
	KindVersion KindVersion
	TypedBags   []*TypedBag
	QueryMeta
}

type KindVersion struct {
	Kind    string
	Version string
}

type TypedBag struct {
	KindVersion KindVersion
	Name        string

	Data []byte

	Meta               map[string]string `json:",omitempty"`
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
	RaftIndex
}
