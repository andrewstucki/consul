package controller

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// Store is the state store interface required for the Controller
type Store interface {
	AbandonCh() <-chan struct{}
	ConfigEntriesByKind(ws memdb.WatchSet, kind string, entMeta *acl.EnterpriseMeta) (uint64, []structs.ConfigEntry, error)
}
