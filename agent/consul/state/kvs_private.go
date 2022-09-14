package state

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-memdb"
)

const (
	tablePrivateKVs        = "private_kvs"
	tablePrivateTombstones = "private_tombstones"
)

const (
	PrivateKVSet            api.KVOp = "private-set"
	PrivateKVDelete         api.KVOp = "private-delete"
	PrivateKVDeleteCAS      api.KVOp = "private-delete-cas"
	PrivateKVDeleteTree     api.KVOp = "private-delete-tree"
	PrivateKVCAS            api.KVOp = "private-cas"
	PrivateKVLock           api.KVOp = "private-lock"
	PrivateKVUnlock         api.KVOp = "private-unlock"
	PrivateKVGet            api.KVOp = "private-get"
	PrivateKVGetOrEmpty     api.KVOp = "private-get-or-empty"
	PrivateKVGetTree        api.KVOp = "private-get-tree"
	PrivateKVCheckSession   api.KVOp = "private-check-session"
	PrivateKVCheckIndex     api.KVOp = "private-check-index"
	PrivateKVCheckNotExists api.KVOp = "private-check-not-exists"
)

// PrivateKVs is used to pull the full list of KVS entries for use during snapshots.
func (s *Snapshot) PrivateKVs() (memdb.ResultIterator, error) {
	return s.tx.Get(tablePrivateKVs, indexID+"_prefix")
}

// PrivateTombstones is used to pull all the tombstones from the graveyard.
func (s *Snapshot) PrivateTombstones() (memdb.ResultIterator, error) {
	return s.store.kvsGraveyard.DumpTxn(s.tx)
}

// PrivateKVS is used when restoring from a snapshot. Use KVSSet for general inserts.
func (s *Restore) PrivateKVS(entry *structs.DirEntry) error {
	if err := insertKVTxn(tablePrivateKVs, s.tx, entry, true, true); err != nil {
		return fmt.Errorf("failed inserting kvs entry: %s", err)
	}

	return nil
}

// PrivateTombstone is used when restoring from a snapshot. For general inserts, use
// Graveyard.InsertTxn.
func (s *Restore) PrivateTombstone(stone *Tombstone) error {
	if err := s.store.privateGraveyard.RestoreTxn(s.tx, stone); err != nil {
		return fmt.Errorf("failed restoring tombstone: %s", err)
	}
	return nil
}

// ReapPrivateTombstones is used to delete all the tombstones with an index
// less than or equal to the given index. This is used to prevent
// unbounded storage growth of the tombstones.
func (s *Store) ReapPrivateTombstones(idx uint64, index uint64) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.privateGraveyard.ReapTxn(tx, index); err != nil {
		return fmt.Errorf("failed to reap kvs tombstones: %s", err)
	}

	return tx.Commit()
}

// PrivateKVSSet is used to store a key/value pair.
func (s *Store) PrivateKVSSet(idx uint64, entry *structs.DirEntry) error {
	entry.EnterpriseMeta.Normalize()
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual set.
	if err := kvsSetTxn(tablePrivateKVs, tx, idx, entry, false); err != nil {
		return err
	}

	return tx.Commit()
}

// PrivateKVSGet is used to retrieve a key/value pair from the state store.
func (s *Store) PrivateKVSGet(ws memdb.WatchSet, key string, entMeta *acl.EnterpriseMeta) (uint64, *structs.DirEntry, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer entMeta
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	return kvsGetTxn(tablePrivateKVs, tablePrivateTombstones, tx, ws, key, *entMeta)
}

// PivateKVSList is used to list out all keys under a given prefix. If the
// prefix is left empty, all keys in the KVS will be returned. The returned
// is the max index of the returned kvs entries or applicable tombstones, or
// else it's the full table indexes for kvs and tombstones.
func (s *Store) PivateKVSList(ws memdb.WatchSet,
	prefix string, entMeta *acl.EnterpriseMeta) (uint64, structs.DirEntries, error) {

	tx := s.db.Txn(false)
	defer tx.Abort()

	// TODO: accept non-pointer entMeta
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	return s.kvsListTxn(tablePrivateKVs, tablePrivateTombstones, s.privateGraveyard, tx, ws, prefix, *entMeta)
}

// PrivateKVSDelete is used to perform a shallow delete on a single key in the
// the state store.
func (s *Store) PrivateKVSDelete(idx uint64, key string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	// Perform the actual delete
	if err := s.kvsDeleteTxn(tablePrivateKVs, s.privateGraveyard, tx, idx, key, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

// PrivateKVSDeleteCAS is used to try doing a KV delete operation with a given
// raft index. If the CAS index specified is not equal to the last
// observed index for the given key, then the call is a noop, otherwise
// a normal KV delete is invoked.
func (s *Store) PrivateKVSDeleteCAS(idx, cidx uint64, key string, entMeta *acl.EnterpriseMeta) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	set, err := s.kvsDeleteCASTxn(tablePrivateKVs, s.privateGraveyard, tx, idx, cidx, key, entMeta)
	if !set || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// PrivateKVSSetCAS is used to do a check-and-set operation on a KV entry. The
// ModifyIndex in the provided entry is used to determine if we should
// write the entry to the state store or bail. Returns a bool indicating
// if a write happened and any error.
func (s *Store) PrivateKVSSetCAS(idx uint64, entry *structs.DirEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	set, err := kvsSetCASTxn(tablePrivateKVs, tx, idx, entry)
	if !set || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// PrivateKVSDeleteTree is used to do a recursive delete on a key prefix
// in the state store. If any keys are modified, the last index is
// set, otherwise this is a no-op.
func (s *Store) PrivateKVSDeleteTree(idx uint64, prefix string, entMeta *acl.EnterpriseMeta) error {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	if err := s.kvsDeleteTreeTxn(tablePrivateKVs, s.privateGraveyard, tx, idx, prefix, entMeta); err != nil {
		return err
	}

	return tx.Commit()
}

// PrivateKVSLock is similar to KVSSet but only performs the set if the lock can be
// acquired.
func (s *Store) PrivateKVSLock(idx uint64, entry *structs.DirEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	locked, err := kvsLockTxn(tablePrivateKVs, tx, idx, entry)
	if !locked || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}

// PrivateKVSUnlock is similar to KVSSet but only performs the set if the lock can be
// unlocked (the key must already exist and be locked).
func (s *Store) PrivateKVSUnlock(idx uint64, entry *structs.DirEntry) (bool, error) {
	tx := s.db.WriteTxn(idx)
	defer tx.Abort()

	unlocked, err := kvsUnlockTxn(tablePrivateKVs, tx, idx, entry)
	if !unlocked || err != nil {
		return false, err
	}

	err = tx.Commit()
	return err == nil, err
}
