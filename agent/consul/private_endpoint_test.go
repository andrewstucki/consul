package consul

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestPrivateKVS_Apply(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	arg := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         state.PrivateKVSet,
		DirEnt: structs.DirEntry{
			Key:   "test",
			Flags: 42,
			Value: []byte("test"),
		},
	}
	var out bool
	if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify
	s := s1.fsm.State()
	_, d, err := s.PrivateKVSGet(nil, "test", &arg.DirEnt.EnterpriseMeta)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d == nil {
		t.Fatalf("should not be nil")
	}

	// Do a check and set
	arg.Op = state.PrivateKVCAS
	arg.DirEnt.ModifyIndex = d.ModifyIndex
	arg.DirEnt.Flags = 43
	if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check this was applied
	if out != true {
		t.Fatalf("bad: %v", out)
	}

	// Verify
	_, d, err = s.PrivateKVSGet(nil, "test", &arg.DirEnt.EnterpriseMeta)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if d.Flags != 43 {
		t.Fatalf("bad: %v", d)
	}
}

func TestPrivateKVS_Apply_ACLDenyxx(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	id := createToken(t, codec, testListRules)

	// Try a write
	argR := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         state.PrivateKVSet,
		DirEnt: structs.DirEntry{
			Key:   "foo/bar",
			Flags: 42,
			Value: []byte("test"),
		},
		WriteRequest: structs.WriteRequest{Token: id},
	}
	var outR bool
	err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &argR, &outR)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// Try a recursive delete
	argR = structs.KVSRequest{
		Datacenter: "dc1",
		Op:         state.PrivateKVDeleteTree,
		DirEnt: structs.DirEntry{
			Key: "test",
		},
		WriteRequest: structs.WriteRequest{Token: id},
	}
	err = msgpackrpc.CallWithCodec(codec, "Private.Apply", &argR, &outR)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}
}

func TestPrivateKVS_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	arg := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         state.PrivateKVSet,
		DirEnt: structs.DirEntry{
			Key:   "test",
			Flags: 42,
			Value: []byte("test"),
		},
	}
	var out bool
	if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	getR := structs.KeyRequest{
		Datacenter: "dc1",
		Key:        "test",
	}
	var dirent structs.IndexedDirEntries
	if err := msgpackrpc.CallWithCodec(codec, "Private.Get", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Entries) != 1 {
		t.Fatalf("Bad: %v", dirent)
	}
	d := dirent.Entries[0]
	if d.Flags != 42 {
		t.Fatalf("bad: %v", d)
	}
	if string(d.Value) != "test" {
		t.Fatalf("bad: %v", d)
	}
}

func TestPrivateKVS_Get_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	arg := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         state.PrivateKVSet,
		DirEnt: structs.DirEntry{
			Key:   "zip",
			Flags: 42,
			Value: []byte("test"),
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out bool
	if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	getR := structs.KeyRequest{
		Datacenter: "dc1",
		Key:        "zip",
	}
	var dirent structs.IndexedDirEntries
	if err := msgpackrpc.CallWithCodec(codec, "Private.Get", &getR, &dirent); !acl.IsErrPermissionDenied(err) {
		t.Fatalf("Expected %v, got err: %v", acl.ErrPermissionDenied, err)
	}

}

func TestPrivateKVS_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	keys := []string{
		"/test/key1",
		"/test/key2",
		"/test/sub/key3",
	}

	for _, key := range keys {
		arg := structs.KVSRequest{
			Datacenter: "dc1",
			Op:         state.PrivateKVSet,
			DirEnt: structs.DirEntry{
				Key:   key,
				Flags: 1,
			},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	getR := structs.KeyRequest{
		Datacenter: "dc1",
		Key:        "/test",
	}
	var dirent structs.IndexedDirEntries
	if err := msgpackrpc.CallWithCodec(codec, "Private.List", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Entries) != 3 {
		t.Fatalf("Bad: %v", dirent.Entries)
	}
	for i := 0; i < len(dirent.Entries); i++ {
		d := dirent.Entries[i]
		if d.Key != keys[i] {
			t.Fatalf("bad: %v", d)
		}
		if d.Flags != 1 {
			t.Fatalf("bad: %v", d)
		}
		if d.Value != nil {
			t.Fatalf("bad: %v", d)
		}
	}
	if dirent.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should not be true")
	}

	// Try listing a nonexistent prefix
	getR.Key = "/nope"
	if err := msgpackrpc.CallWithCodec(codec, "Private.List", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}
	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Entries) != 0 {
		t.Fatalf("Bad: %v", dirent.Entries)
	}
}

func TestPrivateKVS_List_Blocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	keys := []string{
		"/test/key1",
		"/test/key2",
		"/test/sub/key3",
	}

	for _, key := range keys {
		arg := structs.KVSRequest{
			Datacenter: "dc1",
			Op:         state.PrivateKVSet,
			DirEnt: structs.DirEntry{
				Key:   key,
				Flags: 1,
			},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	getR := structs.KeyRequest{
		Datacenter: "dc1",
		Key:        "/test",
	}
	var dirent structs.IndexedDirEntries
	if err := msgpackrpc.CallWithCodec(codec, "Private.List", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Setup a blocking query
	getR.MinQueryIndex = dirent.Index
	getR.MaxQueryTime = time.Second

	// Async cause a change
	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		codec := rpcClient(t, s1)
		defer codec.Close()
		arg := structs.KVSRequest{
			Datacenter: "dc1",
			Op:         state.PrivateKVDelete,
			DirEnt: structs.DirEntry{
				Key: "/test/sub/key3",
			},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
			t.Errorf("RPC call failed: %v", err)
		}
	}()

	// Re-run the query
	dirent = structs.IndexedDirEntries{}
	if err := msgpackrpc.CallWithCodec(codec, "Private.List", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block at least 100ms
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Entries) != 2 {
		for _, ent := range dirent.Entries {
			t.Errorf("Bad: %#v", *ent)
		}
	}
	for i := 0; i < len(dirent.Entries); i++ {
		d := dirent.Entries[i]
		if d.Key != keys[i] {
			t.Fatalf("bad: %v", d)
		}
		if d.Flags != 1 {
			t.Fatalf("bad: %v", d)
		}
		if d.Value != nil {
			t.Fatalf("bad: %v", d)
		}
	}
}

func TestPrivateKVS_List_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	keys := []string{
		"abe",
		"bar",
		"foo",
		"test",
		"zip",
	}

	for _, key := range keys {
		arg := structs.KVSRequest{
			Datacenter: "dc1",
			Op:         state.PrivateKVSet,
			DirEnt: structs.DirEntry{
				Key:   key,
				Flags: 1,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	id := createToken(t, codec, testListRules)

	getR := structs.KeyRequest{
		Datacenter:   "dc1",
		Key:          "",
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var dirent structs.IndexedDirEntries
	if err := msgpackrpc.CallWithCodec(codec, "Private.List", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Entries) != 2 {
		t.Fatalf("Bad: %v", dirent.Entries)
	}
	for i := 0; i < len(dirent.Entries); i++ {
		d := dirent.Entries[i]
		switch i {
		case 0:
			if d.Key != "foo" {
				t.Fatalf("bad key")
			}
		case 1:
			if d.Key != "test" {
				t.Fatalf("bad key")
			}
		}
	}
	if !dirent.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should be true")
	}
}

func TestPrivateKVS_List_ACLEnableKeyListPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
		c.ACLEnableKeyListPolicy = true
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	keys := []string{
		"abe",
		"bar/bar1",
		"bar/bar2",
		"zip",
	}

	for _, key := range keys {
		arg := structs.KVSRequest{
			Datacenter: "dc1",
			Op:         state.PrivateKVSet,
			DirEnt: structs.DirEntry{
				Key:   key,
				Flags: 1,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	//write acl policy that denies recursive reads on ""
	var testListRules1 = `
key_prefix "" {
	policy = "deny"
}
key_prefix "bar" {
	policy = "list"
}
key_prefix "zip" {
	policy = "read"
}
`

	id := createToken(t, codec, testListRules1)

	//recursive read on empty prefix should fail
	getReq := structs.KeyRequest{
		Datacenter:   "dc1",
		Key:          "",
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var dirent structs.IndexedDirEntries
	if err := msgpackrpc.CallWithCodec(codec, "Private.List", &getReq, &dirent); !acl.IsErrPermissionDenied(err) {
		t.Fatalf("expected %v but got err: %v", acl.ErrPermissionDenied, err)
	}

	//listing keys on empty prefix should fail
	getKeysReq := structs.KeyListRequest{
		Datacenter:   "dc1",
		Prefix:       "",
		Seperator:    "/",
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var keyList structs.IndexedKeyList
	if err := msgpackrpc.CallWithCodec(codec, "Private.ListKeys", &getKeysReq, &keyList); !acl.IsErrPermissionDenied(err) {
		t.Fatalf("expected %v but got err: %v", acl.ErrPermissionDenied, err)
	}

	// recursive read with a prefix that has list permissions should succeed
	getReq2 := structs.KeyRequest{
		Datacenter:   "dc1",
		Key:          "bar",
		QueryOptions: structs.QueryOptions{Token: id},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Private.List", &getReq2, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	expectedKeys := []string{"bar/bar1", "bar/bar2"}
	var actualKeys []string
	for _, entry := range dirent.Entries {
		actualKeys = append(actualKeys, entry.Key)
	}

	require.Equal(t, expectedKeys, actualKeys)

	// list keys with a prefix that has list permissions should succeed
	getKeysReq2 := structs.KeyListRequest{
		Datacenter:   "dc1",
		Prefix:       "bar",
		QueryOptions: structs.QueryOptions{Token: id},
	}
	if err := msgpackrpc.CallWithCodec(codec, "Private.ListKeys", &getKeysReq2, &keyList); err != nil {
		t.Fatalf("err: %v", err)
	}

	actualKeys = keyList.Keys

	require.Equal(t, expectedKeys, actualKeys)

}

func TestPrivateKVS_ListKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	keys := []string{
		"/test/key1",
		"/test/key2",
		"/test/sub/key3",
	}

	for _, key := range keys {
		arg := structs.KVSRequest{
			Datacenter: "dc1",
			Op:         state.PrivateKVSet,
			DirEnt: structs.DirEntry{
				Key:   key,
				Flags: 1,
			},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	getR := structs.KeyListRequest{
		Datacenter: "dc1",
		Prefix:     "/test/",
		Seperator:  "/",
	}
	var dirent structs.IndexedKeyList
	if err := msgpackrpc.CallWithCodec(codec, "Private.ListKeys", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Keys) != 3 {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
	if dirent.Keys[0] != "/test/key1" {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
	if dirent.Keys[1] != "/test/key2" {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
	if dirent.Keys[2] != "/test/sub/" {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
	if dirent.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should not be true")
	}

	// Try listing a nonexistent prefix
	getR.Prefix = "/nope"
	if err := msgpackrpc.CallWithCodec(codec, "Private.ListKeys", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}
	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Keys) != 0 {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
}

func TestPrivateKVS_ListKeys_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.PrimaryDatacenter = "dc1"
		c.ACLsEnabled = true
		c.ACLInitialManagementToken = "root"
		c.ACLResolverSettings.ACLDefaultPolicy = "deny"
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))

	keys := []string{
		"abe",
		"bar",
		"foo",
		"test",
		"zip",
	}

	for _, key := range keys {
		arg := structs.KVSRequest{
			Datacenter: "dc1",
			Op:         state.PrivateKVSet,
			DirEnt: structs.DirEntry{
				Key:   key,
				Flags: 1,
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	id := createToken(t, codec, testListRules)

	getR := structs.KeyListRequest{
		Datacenter:   "dc1",
		Prefix:       "",
		Seperator:    "/",
		QueryOptions: structs.QueryOptions{Token: id},
	}
	var dirent structs.IndexedKeyList
	if err := msgpackrpc.CallWithCodec(codec, "Private.ListKeys", &getR, &dirent); err != nil {
		t.Fatalf("err: %v", err)
	}

	if dirent.Index == 0 {
		t.Fatalf("Bad: %v", dirent)
	}
	if len(dirent.Keys) != 2 {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
	if dirent.Keys[0] != "foo" {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
	if dirent.Keys[1] != "test" {
		t.Fatalf("Bad: %v", dirent.Keys)
	}
	if !dirent.QueryMeta.ResultsFilteredByACLs {
		t.Fatal("ResultsFilteredByACLs should be true")
	}
}

func TestPrivateKVS_Apply_LockDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForTestAgent(t, s1.RPC, "dc1")

	// Create and invalidate a session with a lock.
	s := s1.fsm.State()

	if err := s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	session := &structs.Session{
		ID:        generateUUID(),
		Node:      "foo",
		LockDelay: 50 * time.Millisecond,
	}
	if err := s.SessionCreate(2, session); err != nil {
		t.Fatalf("err: %v", err)
	}
	id := session.ID
	d := &structs.DirEntry{
		Key:     "test",
		Session: id,
	}
	if ok, err := s.PrivateKVSLock(3, d); err != nil || !ok {
		t.Fatalf("err: %v", err)
	}

	if err := s.SessionDestroy(4, id, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make a new session that is valid.
	if err := s.SessionCreate(5, session); err != nil {
		t.Fatalf("err: %v", err)
	}
	validID := session.ID

	// Make a lock request.
	arg := structs.KVSRequest{
		Datacenter: "dc1",
		Op:         state.PrivateKVLock,
		DirEnt: structs.DirEntry{
			Key:     "test",
			Session: validID,
		},
	}
	var out bool
	if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != false {
		t.Fatalf("should not acquire")
	}

	// Wait for lock-delay.
	time.Sleep(50 * time.Millisecond)

	// Should acquire.
	if err := msgpackrpc.CallWithCodec(codec, "Private.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != true {
		t.Fatalf("should acquire")
	}
}
