package controller

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// Store is the state store interface required for the Controller
type Store interface {
	AbandonCh() <-chan struct{}
	EnsureConfigEntryCAS(idx, cidx uint64, conf structs.ConfigEntry) (bool, error)
	ConfigEntry(ws memdb.WatchSet, kind string, name string, entMeta *acl.EnterpriseMeta) (uint64, structs.ConfigEntry, error)
	ControlledUpdateConfigEntry(idx uint64, conf structs.ControlledConfigEntry) (bool, error)
	DeleteConfigEntryCAS(idx uint64, cidx uint64, conf structs.ConfigEntry) (bool, error)
	ConfigEntriesByKind(ws memdb.WatchSet, kind string, entMeta *acl.EnterpriseMeta) (uint64, []structs.ConfigEntry, error)
}
