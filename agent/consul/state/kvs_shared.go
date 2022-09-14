package state

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// newKVTableSchema returns a factory for a table schema used for storing structs.DirEntry
func newKVTableSchema(table string) func() *memdb.TableSchema {
	return func() *memdb.TableSchema {
		return &memdb.TableSchema{
			Name: table,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:         indexID,
					AllowMissing: false,
					Unique:       true,
					Indexer:      kvsIndexer(),
				},
				indexSession: {
					Name:         indexSession,
					AllowMissing: true,
					Unique:       false,
					Indexer: &memdb.UUIDFieldIndex{
						Field: "Session",
					},
				},
			},
		}
	}
}

// // indexFromIDValue creates an index key from any struct that implements singleValueID
// func indexFromIDValue(e singleValueID) ([]byte, error) {
// 	v := e.IDValue()
// 	if v == "" {
// 		return nil, errMissingValueForIndex
// 	}

// 	var b indexBuilder
// 	b.String(v)
// 	return b.Bytes(), nil
// }

// newTombstonesTableSchema returns a factory for a table schema used for storing tombstones
// during KV delete operations to prevent the index from sliding backwards.
func newTombstonesTableSchema(table string) func() *memdb.TableSchema {
	return func() *memdb.TableSchema {
		return &memdb.TableSchema{
			Name: table,
			Indexes: map[string]*memdb.IndexSchema{
				indexID: {
					Name:         indexID,
					AllowMissing: false,
					Unique:       true,
					Indexer:      kvsIndexer(),
				},
			},
		}
	}
}

// indexFromIDValue creates an index key from any struct that implements singleValueID
func indexFromIDValue(e singleValueID) ([]byte, error) {
	v := e.IDValue()
	if v == "" {
		return nil, errMissingValueForIndex
	}

	var b indexBuilder
	b.String(v)
	return b.Bytes(), nil
}

// kvsSetTxn is used to insert or update a key/value pair in the state
// store. It is the inner method used and handles only the actual storage.
// If updateSession is true, then the incoming entry will set the new
// session (should be validated before calling this). Otherwise, we will keep
// whatever the existing session is.
func kvsSetTxn(table string, tx WriteTxn, idx uint64, entry *structs.DirEntry, updateSession bool) error {
	existingNode, err := tx.First(table, indexID, entry)
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}
	existing, _ := existingNode.(*structs.DirEntry)

	// Set the CreateIndex.
	if existing != nil {
		entry.CreateIndex = existing.CreateIndex
	} else {
		entry.CreateIndex = idx
	}

	// Preserve the existing session unless told otherwise. The "existing"
	// session for a new entry is "no session".
	if !updateSession {
		if existing != nil {
			entry.Session = existing.Session
		} else {
			entry.Session = ""
		}
	}

	// Set the ModifyIndex.
	if existing != nil && existing.Equal(entry) {
		// Skip further writing in the state store if the entry is not actually
		// changed. Nevertheless, the input's ModifyIndex should be reset
		// since the TXN API returns a copy in the response.
		entry.ModifyIndex = existing.ModifyIndex
		return nil
	}
	entry.ModifyIndex = idx

	// Store the kv pair in the state store and update the index.
	if err := insertKVTxn(table, tx, entry, false, false); err != nil {
		return fmt.Errorf("failed inserting kvs entry: %s", err)
	}

	return nil
}

// kvsGetTxn is the inner method that gets a KVS entry inside an existing
// transaction.
func kvsGetTxn(table, tombstones string, tx ReadTxn,
	ws memdb.WatchSet, key string, entMeta acl.EnterpriseMeta) (uint64, *structs.DirEntry, error) {

	// Get the table index.
	idx := kvsMaxIndex(table, tombstones, tx, entMeta)

	watchCh, entry, err := tx.FirstWatch(table, indexID, Query{Value: key, EnterpriseMeta: entMeta})
	if err != nil {
		return 0, nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	ws.Add(watchCh)
	if entry != nil {
		return idx, entry.(*structs.DirEntry), nil
	}
	return idx, nil, nil
}

// kvsListTxn is the inner method that gets a list of KVS entries matching a
// prefix.
func (s *Store) kvsListTxn(table, tombstones string, graveyard *Graveyard, tx ReadTxn,
	ws memdb.WatchSet, prefix string, entMeta acl.EnterpriseMeta) (uint64, structs.DirEntries, error) {

	// Get the table indexes.
	idx := kvsMaxIndex(table, tombstones, tx, entMeta)

	lindex, entries, err := kvsListEntriesTxn(table, tx, ws, prefix, entMeta)
	if err != nil {
		return 0, nil, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Check for the highest index in the graveyard. If the prefix is empty
	// then just use the full table indexes since we are listing everything.
	if prefix != "" {
		gindex, err := graveyard.GetMaxIndexTxn(tx, prefix, &entMeta)
		if err != nil {
			return 0, nil, fmt.Errorf("failed graveyard lookup: %s", err)
		}
		if gindex > lindex {
			lindex = gindex
		}
	} else {
		lindex = idx
	}

	// Use the sub index if it was set and there are entries, otherwise use
	// the full table index from above.
	if lindex != 0 {
		idx = lindex
	}
	return idx, entries, nil
}

// kvsDeleteTxn is the inner method used to perform the actual deletion
// of a key/value pair within an existing transaction.
func (s *Store) kvsDeleteTxn(table string, graveyard *Graveyard, tx WriteTxn, idx uint64, key string, entMeta *acl.EnterpriseMeta) error {

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	// Look up the entry in the state store.
	entry, err := tx.First(table, indexID, Query{Value: key, EnterpriseMeta: *entMeta})
	if err != nil {
		return fmt.Errorf("failed kvs lookup: %s", err)
	}
	if entry == nil {
		return nil
	}

	// Create a tombstone.
	if err := graveyard.InsertTxn(tx, key, idx, entMeta); err != nil {
		return fmt.Errorf("failed adding to graveyard: %s", err)
	}

	return kvsDeleteWithEntry(table, tx, entry.(*structs.DirEntry), idx)
}

// kvsDeleteCASTxn is the inner method that does a CAS delete within an existing
// transaction.
func (s *Store) kvsDeleteCASTxn(table string, graveyard *Graveyard, tx WriteTxn, idx, cidx uint64, key string, entMeta *acl.EnterpriseMeta) (bool, error) {
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}
	entry, err := tx.First(table, indexID, Query{Value: key, EnterpriseMeta: *entMeta})
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// If the existing index does not match the provided CAS
	// index arg, then we shouldn't update anything and can safely
	// return early here.
	e, ok := entry.(*structs.DirEntry)
	if !ok || e.ModifyIndex != cidx {
		return entry == nil, nil
	}

	// Call the actual deletion if the above passed.
	if err := s.kvsDeleteTxn(table, graveyard, tx, idx, key, entMeta); err != nil {
		return false, err
	}
	return true, nil
}

// kvsSetCASTxn is the inner method used to do a CAS inside an existing
// transaction.
func kvsSetCASTxn(table string, tx WriteTxn, idx uint64, entry *structs.DirEntry) (bool, error) {
	existing, err := tx.First(table, indexID, entry)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Check if the we should do the set. A ModifyIndex of 0 means that
	// we are doing a set-if-not-exists.
	if entry.ModifyIndex == 0 && existing != nil {
		return false, nil
	}
	if entry.ModifyIndex != 0 && existing == nil {
		return false, nil
	}
	e, ok := existing.(*structs.DirEntry)
	if ok && entry.ModifyIndex != 0 && entry.ModifyIndex != e.ModifyIndex {
		return false, nil
	}

	// If we made it this far, we should perform the set.
	if err := kvsSetTxn(table, tx, idx, entry, false); err != nil {
		return false, err
	}
	return true, nil
}

// kvsLockTxn is the inner method that does a lock inside an existing
// transaction.
func kvsLockTxn(table string, tx WriteTxn, idx uint64, entry *structs.DirEntry) (bool, error) {
	// Verify that a session is present.
	if entry.Session == "" {
		return false, fmt.Errorf("missing session")
	}

	// Verify that the session exists.
	sess, err := tx.First(tableSessions, indexID, Query{Value: entry.Session, EnterpriseMeta: entry.EnterpriseMeta})
	if err != nil {
		return false, fmt.Errorf("failed session lookup: %s", err)
	}
	if sess == nil {
		return false, fmt.Errorf("invalid session %#v", entry.Session)
	}

	existing, err := tx.First(table, indexID, entry)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Set up the entry, using the existing entry if present.
	if existing != nil {
		e := existing.(*structs.DirEntry)
		if e.Session == entry.Session {
			// We already hold this lock, good to go.
			entry.CreateIndex = e.CreateIndex
			entry.LockIndex = e.LockIndex
		} else if e.Session != "" {
			// Bail out, someone else holds this lock.
			return false, nil
		} else {
			// Set up a new lock with this session.
			entry.CreateIndex = e.CreateIndex
			entry.LockIndex = e.LockIndex + 1
		}
	} else {
		entry.CreateIndex = idx
		entry.LockIndex = 1
	}
	entry.ModifyIndex = idx

	// If we made it this far, we should perform the set.
	if err := kvsSetTxn(table, tx, idx, entry, true); err != nil {
		return false, err
	}
	return true, nil
}

// kvsUnlockTxn is the inner method that does an unlock inside an existing
// transaction.
func kvsUnlockTxn(table string, tx WriteTxn, idx uint64, entry *structs.DirEntry) (bool, error) {
	// Verify that a session is present.
	if entry.Session == "" {
		return false, fmt.Errorf("missing session")
	}

	existing, err := tx.First(table, indexID, entry)
	if err != nil {
		return false, fmt.Errorf("failed kvs lookup: %s", err)
	}

	// Bail if there's no existing key.
	if existing == nil {
		return false, nil
	}

	// Make sure the given session is the lock holder.
	e := existing.(*structs.DirEntry)
	if e.Session != entry.Session {
		return false, nil
	}

	// Clear the lock and update the entry.
	entry.Session = ""
	entry.LockIndex = e.LockIndex
	entry.CreateIndex = e.CreateIndex
	entry.ModifyIndex = idx

	// If we made it this far, we should perform the set.
	if err := kvsSetTxn(table, tx, idx, entry, true); err != nil {
		return false, err
	}
	return true, nil
}

// kvsCheckSessionTxn checks to see if the given session matches the current
// entry for a key.
func kvsCheckSessionTxn(table string, tx WriteTxn,
	key string, session string, entMeta *acl.EnterpriseMeta) (*structs.DirEntry, error) {

	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	entry, err := tx.First(table, indexID, Query{Value: key, EnterpriseMeta: *entMeta})
	if err != nil {
		return nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	if entry == nil {
		return nil, fmt.Errorf("failed to check session, key %q doesn't exist", key)
	}

	e := entry.(*structs.DirEntry)
	if e.Session != session {
		return nil, fmt.Errorf("failed session check for key %q, current session %q != %q", key, e.Session, session)
	}

	return e, nil
}

// kvsCheckIndexTxn checks to see if the given modify index matches the current
// entry for a key.
func kvsCheckIndexTxn(table string, tx WriteTxn,
	key string, cidx uint64, entMeta acl.EnterpriseMeta) (*structs.DirEntry, error) {

	entry, err := tx.First(table, indexID, Query{Value: key, EnterpriseMeta: entMeta})
	if err != nil {
		return nil, fmt.Errorf("failed kvs lookup: %s", err)
	}
	if entry == nil {
		return nil, fmt.Errorf("failed to check index, key %q doesn't exist", key)
	}

	e := entry.(*structs.DirEntry)
	if e.ModifyIndex != cidx {
		return nil, fmt.Errorf("failed index check for key %q, current modify index %d != %d", key, e.ModifyIndex, cidx)
	}

	return e, nil
}

// KVSLockDelay returns the expiration time for any lock delay associated with
// the given key.
func (s *Store) KVSLockDelay(key string, entMeta *acl.EnterpriseMeta) time.Time {
	return s.lockDelay.GetExpiration(key, entMeta)
}
