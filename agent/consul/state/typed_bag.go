package state

import (
	"fmt"
	"strings"

	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

const (
	tableTypedBags = "typed_bags"
)

func typedBagSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableTypedBags,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix[any, *structs.TypedBag, any]{
					readIndex:   indexFromTypeBagKindVersion,
					writeIndex:  indexFromTypedBag,
					prefixIndex: indexFromTypeBagKindVersion,
				},
			},
		},
	}
}

type KindVersionQuery struct {
	Kind string
	Version string
	acl.EnterpriseMeta
}

type KindVersionNameQuery struct {
	Kind string
	Version string
	Name string
	acl.EnterpriseMeta
}

func indexFromTypeBagKindVersion(arg interface{}) ([]byte, error) {
	var b indexBuilder

	switch n := arg.(type) {
	case *acl.EnterpriseMeta:
		return nil, nil
	case acl.EnterpriseMeta:
		return b.Bytes(), nil
	case KindVersionQuery:
		b.String(strings.ToLower(n.Kind))
		b.String(strings.ToLower(n.Version))
		return b.Bytes(), nil
	case KindVersionNameQuery:
		b.String(strings.ToLower(n.Kind))
		b.String(strings.ToLower(n.Version))
		b.String(strings.ToLower(n.Name))
		return b.Bytes(), nil
	}

	return nil, fmt.Errorf("invalid type for KindVersionQuery: %T", arg)
}

func indexFromTypedBag(c *structs.TypedBag) ([]byte, error) {
	var b indexBuilder
	b.String(strings.ToLower(c.KindVersion.Kind))
	b.String(strings.ToLower(c.KindVersion.Version))
	b.String(strings.ToLower(c.Name))
	return b.Bytes(), nil
}

func (s *Snapshot) TypedBags() ([]*structs.TypedBag, error) {
	entries, err := s.tx.Get(tableTypedBags, indexID)
	if err != nil {
		return nil, err
	}

	var ret []*structs.TypedBag
	for wrapped := entries.Next(); wrapped != nil; wrapped = entries.Next() {
		ret = append(ret, wrapped.(*structs.TypedBag))
	}

	return ret, nil
}

func (s *Restore) TypedBag(b *structs.TypedBag) error {
	return insertTypedBagWithTxn(s.tx, b.RaftIndex.ModifyIndex, b)
}

func insertTypedBagWithTxn(tx WriteTxn, idx uint64, bag *structs.TypedBag) error {
	if bag == nil {
		return nil
	}

	// Insert the config entry and update the index
	if err := tx.Insert(tableTypedBags, bag); err != nil {
		return fmt.Errorf("failed inserting typed bag: %s", err)
	}
	if err := indexUpdateMaxTxn(tx, idx, tableTypedBags); err != nil {
		return fmt.Errorf("failed updating index: %v", err)
	}

	return nil
}


func (s *Store) EnsureTypedBag(idx uint64, bag *structs.TypedBag) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := ensureTypedBagTxn(tx, idx, bag); err != nil {
		return err
	}

	return tx.Commit()
}

func ensureTypedBagTxn(tx WriteTxn, idx uint64, bag *structs.TypedBag) error {
	existing, err := tx.First(tableTypedBags, indexID, KindVersionNameQuery{
		Kind: bag.KindVersion.Kind,
		Version: bag.KindVersion.Version,
		Name: bag.Name,
		EnterpriseMeta: bag.EnterpriseMeta,
	})
	if err != nil {
		return fmt.Errorf("failed typed bag lookup: %s", err)
	}

	raftIndex := bag.RaftIndex
	if existing != nil {
		existingIdx := existing.(*structs.TypedBag).RaftIndex
		raftIndex.CreateIndex = existingIdx.CreateIndex
	} else {
		raftIndex.CreateIndex = idx
	}
	raftIndex.ModifyIndex = idx

	return insertTypedBagWithTxn(tx, idx, bag)
}

func (s *Store) TypedBag(ws memdb.WatchSet, kindVersion structs.KindVersion, name string, entMeta acl.EnterpriseMeta) (uint64, *structs.TypedBag, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return typedBagTxn(tx, ws, kindVersion.Kind, kindVersion.Version, name, entMeta)
}

func typedBagTxn(tx ReadTxn, ws memdb.WatchSet, kind, version, name string, entMeta acl.EnterpriseMeta) (uint64, *structs.TypedBag, error) {
	// Get the index
	idx := maxIndexTxn(tx, tableTypedBags)

	// Get the existing bag.
	watchCh, existing, err := tx.FirstWatch(tableTypedBags, indexID, KindVersionNameQuery{
		Kind: kind,
		Version: version,
		Name: name,
		EnterpriseMeta: entMeta,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("failed typed bag lookup: %s", err)
	}
	ws.Add(watchCh)
	if existing == nil {
		return idx, nil, nil
	}

	bag, ok := existing.(*structs.TypedBag)
	if !ok {
		return 0, nil, fmt.Errorf("typed bag %q (%s/%s) is an invalid type: %T", name, kind, version, bag)
	}

	return idx, bag, nil
}

func (s *Store) TypedBags(ws memdb.WatchSet) (uint64, []*structs.TypedBag, error) {
	return s.TypedBagsByKindVersion(ws, structs.KindVersion{})
}

func (s *Store) TypedBagsByKindVersion(ws memdb.WatchSet, kindVersion structs.KindVersion) (uint64, []*structs.TypedBag, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	return typedBagsByKindTxn(tx, ws, kindVersion.Kind, kindVersion.Version)
}

func getAllTypedBagsWithTxn(tx ReadTxn) (memdb.ResultIterator, error) {
	return tx.Get(tableTypedBags, indexID)
}

func getAllTypedBagsByKindVersionWithTxn(tx ReadTxn, kind, version string) (memdb.ResultIterator, error) {
	return getTypedBagKindVersionsWithTxn(tx, kind, version)
}

func getTypedBagKindVersionsWithTxn(tx ReadTxn, kind, version string) (memdb.ResultIterator, error) {
	return tx.Get(tableTypedBags, indexID+"_prefix", KindVersionQuery{Kind: kind, Version: version})
}

func typedBagsByKindTxn(tx ReadTxn, ws memdb.WatchSet, kind, version string) (uint64, []*structs.TypedBag, error) {
	// Get the index and watch for updates
	idx := maxIndexWatchTxn(tx, ws, tableTypedBags)

	// Lookup by kind, or all if kind is empty
	var iter memdb.ResultIterator
	var err error
	if kind != "" && version != "" {
		iter, err = getTypedBagKindVersionsWithTxn(tx, kind, version)
	} else {
		iter, err = getAllTypedBagsWithTxn(tx)
	}
	if err != nil {
		return 0, nil, fmt.Errorf("failed typed bag lookup: %s", err)
	}
	ws.Add(iter.WatchCh())

	var results []*structs.TypedBag
	for v := iter.Next(); v != nil; v = iter.Next() {
		results = append(results, v.(*structs.TypedBag))
	}
	return idx, results, nil
}


func (s *Store) DeleteTypedBag(idx uint64, kindVersion structs.KindVersion, name string, entMeta acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := deleteTypedBagTxn(tx, idx, kindVersion.Kind, kindVersion.Version, name, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

func deleteTypedBagTxn(tx WriteTxn, idx uint64, kind, version, name string, entMeta acl.EnterpriseMeta) error {	
	existing, err := tx.First(tableTypedBags, indexID, KindVersionNameQuery{
		Kind: kind,
		Version: version,
		Name: name,
		EnterpriseMeta: entMeta,
	})
	if err != nil {
		return fmt.Errorf("failed config entry lookup: %s", err)
	}
	if existing == nil {
		return nil
	}

	// Delete the bag from the DB and update the index.
	if err := tx.Delete(tableTypedBags, existing); err != nil {
		return fmt.Errorf("failed removing typed bag: %s", err)
	}
	if err := tx.Insert(tableIndex, &IndexEntry{tableTypedBags, idx}); err != nil {
		return fmt.Errorf("failed updating index: %s", err)
	}

	return nil
}