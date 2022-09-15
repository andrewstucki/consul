package consul

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
)

func TestTypedBag_Apply(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testrpc.WaitForLeader(t, s1.RPC, "dc1")

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	testutil.RunStep(t, "send the apply request", func(t *testing.T) {
		updated := structs.TypedBag{
			KindVersion: kindVersion,
			Name:        "foo",
		}
		args := structs.TypedBagRequest{
			Op:  structs.TypedBagUpsert,
			Bag: updated,
		}
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.Apply", &args, &out))
		require.True(t, out)
	})

	testutil.RunStep(t, "verify the entry was updated", func(t *testing.T) {
		// the previous RPC should not return until the primary has been updated but will return
		// before the secondary has the data.
		_, bag, err := s1.fsm.State().TypedBag(nil, kindVersion, "foo", acl.EnterpriseMeta{})
		require.NoError(t, err)

		require.Equal(t, "foo", bag.Name)
		require.Equal(t, kindVersion, bag.KindVersion)
	})

	testutil.RunStep(t, "update the entry again", func(t *testing.T) {
		updated := structs.TypedBag{
			KindVersion: kindVersion,
			Name:        "foo",
			Data:        []byte("bar"),
		}

		args := structs.TypedBagRequest{
			Op:         structs.TypedBagUpsert,
			Datacenter: "dc1",
			Bag:        updated,
		}

		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.Apply", &args, &out))
		require.True(t, out)
	})

	testutil.RunStep(t, "verify the entry was updated in the state store", func(t *testing.T) {
		_, bag, err := s1.fsm.State().TypedBag(nil, kindVersion, "foo", acl.EnterpriseMeta{})
		require.NoError(t, err)

		require.Equal(t, kindVersion, bag.KindVersion)
		require.Equal(t, "foo", bag.Name)
		require.Equal(t, []byte("bar"), bag.Data)
	})

	testutil.RunStep(t, "delete the entry", func(t *testing.T) {
		updated := structs.TypedBag{
			KindVersion: kindVersion,
			Name:        "foo",
		}

		args := structs.TypedBagRequest{
			Op:         structs.TypedBagDelete,
			Datacenter: "dc1",
			Bag:        updated,
		}

		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.Apply", &args, &out))
		require.True(t, out)
	})

	testutil.RunStep(t, "verify the entry is deleted", func(t *testing.T) {
		_, bag, err := s1.fsm.State().TypedBag(nil, kindVersion, "foo", acl.EnterpriseMeta{})
		require.NoError(t, err)
		require.Nil(t, bag)
	})
}

func TestTypedBag_Apply_ACLDeny(t *testing.T) {
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
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	rules := `
	mesh = "write"
	`
	id := createToken(t, codec, rules)

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	// This should fail since we don't have mesh perms
	args := structs.TypedBagRequest{
		Op:         structs.TypedBagUpsert,
		Datacenter: "dc1",
		Bag: structs.TypedBag{
			KindVersion: kindVersion,
			Name:        "foo",
		},
	}
	out := false
	err := msgpackrpc.CallWithCodec(codec, "TypedBag.Apply", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The mesh perms should work
	args.WriteRequest.Token = id
	err = msgpackrpc.CallWithCodec(codec, "TypedBag.Apply", &args, &out)
	require.NoError(t, err)

	state := s1.fsm.State()
	_, bag, err := state.TypedBag(nil, kindVersion, "foo", acl.EnterpriseMeta{})
	require.NoError(t, err)

	require.Equal(t, "foo", bag.Name)
	require.Equal(t, kindVersion, bag.KindVersion)
}

func TestTypedBag_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	// Create a dummy service in the state store to look up.
	bag := &structs.TypedBag{
		KindVersion: kindVersion,
		Name:        "foo",
	}
	state := s1.fsm.State()
	require.NoError(t, state.EnsureTypedBag(1, bag))

	args := structs.TypedBagQuery{
		KindVersion: kindVersion,
		Name:        "foo",
		Datacenter:  s1.config.Datacenter,
	}
	var out structs.TypedBagResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.Get", &args, &out))

	require.Equal(t, "foo", out.TypedBag.Name)
	require.Equal(t, kindVersion, out.TypedBag.KindVersion)
}

func TestTypedBag_Get_BlockOnNonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
	})

	codec := rpcClient(t, s1)

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	{ // create one relevant entry
		var out bool
		require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.Apply", &structs.TypedBagRequest{
			Bag: structs.TypedBag{
				KindVersion: kindVersion,
				Name:        "foo",
			},
			Op: structs.TypedBagUpsert,
		}, &out))
		require.True(t, out)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := structs.TypedBagQuery{
					KindVersion: kindVersion,
					Name:        "does-not-exist",
				}
				args.QueryOptions.MinQueryIndex = minQueryIndex

				var out structs.TypedBagResponse
				errCh := channelCallRPC(s1, "TypedBag.Get", &args, &out, nil)
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out bool
				return channelCallRPC(s1, "TypedBag.Apply", &structs.TypedBagRequest{
					Bag: structs.TypedBag{
						KindVersion: kindVersion,
						Name:        fmt.Sprintf("other%d", i),
					},
					Op: structs.TypedBagUpsert,
				}, &out, func() error {
					if !out {
						return fmt.Errorf("[%d] unexpectedly returned false", i)
					}
					return nil
				})
			},
		)
	})
}

func TestTypedBag_Get_ACLDeny(t *testing.T) {
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
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	rules := `
	mesh = "write"
	`

	id := createToken(t, codec, rules)

	// Create some dummy records to be looked up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureTypedBag(1, &structs.TypedBag{
		KindVersion: kindVersion,
		Name:        "foo",
	}))

	// This should fail since we don't have mesh perms.
	args := structs.TypedBagQuery{
		Datacenter:  s1.config.Datacenter,
		KindVersion: kindVersion,
		Name:        "foo",
	}
	var out structs.TypedBagResponse
	err := msgpackrpc.CallWithCodec(codec, "TypedBag.Get", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// The "foo" service should work.
	args.QueryOptions.Token = id
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.Get", &args, &out))

	require.Equal(t, "foo", out.TypedBag.Name)
	require.Equal(t, kindVersion, out.TypedBag.KindVersion)
}

func TestTypedBag_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	// Create some dummy services in the state store to look up.
	state := s1.fsm.State()

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	expected := structs.TypedBagListResponse{
		TypedBags: []*structs.TypedBag{
			&structs.TypedBag{
				KindVersion: kindVersion,
				Name:        "bar",
			},
			&structs.TypedBag{
				KindVersion: kindVersion,
				Name:        "foo",
			},
		},
	}
	require.NoError(t, state.EnsureTypedBag(1, expected.TypedBags[0]))
	require.NoError(t, state.EnsureTypedBag(2, expected.TypedBags[1]))

	args := structs.TypedBagQuery{
		KindVersion: kindVersion,
		Datacenter:  "dc1",
	}
	var out structs.TypedBagListResponse
	require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.List", &args, &out))

	expected.KindVersion = kindVersion
	expected.QueryMeta = out.QueryMeta
	require.Equal(t, expected, out)
}

func TestTypedBag_List_BlockOnNoChange(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	_, s1 := testServerWithConfig(t, func(c *Config) {
		c.DevMode = true // keep it in ram to make it 10x faster on macos
	})

	codec := rpcClient(t, s1)

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	run := func(t *testing.T, dataPrefix string) {
		rpcBlockingQueryTestHarness(t,
			func(minQueryIndex uint64) (*structs.QueryMeta, <-chan error) {
				args := structs.TypedBagQuery{
					KindVersion: kindVersion,
					Datacenter:  "dc1",
				}
				args.QueryOptions.MinQueryIndex = minQueryIndex

				var out structs.TypedBagListResponse

				errCh := channelCallRPC(s1, "TypedBag.List", &args, &out, nil)
				return &out.QueryMeta, errCh
			},
			func(i int) <-chan error {
				var out bool
				return channelCallRPC(s1, "TypedBag.Apply", &structs.TypedBagRequest{
					Op: structs.TypedBagUpsert,
					Bag: structs.TypedBag{
						KindVersion: structs.KindVersion{
							Kind:    "other",
							Version: "2",
						},
						Name: fmt.Sprintf(dataPrefix+"%d", i),
					},
				}, &out, func() error {
					if !out {
						return fmt.Errorf("[%d] unexpectedly returned false", i)
					}
					return nil
				})
			},
		)
	}

	testutil.RunStep(t, "test the errNotFound path", func(t *testing.T) {
		run(t, "other")
	})

	{ // Create some dummy services in the state store to look up.
		for _, bag := range []structs.TypedBag{
			structs.TypedBag{
				KindVersion: kindVersion,
				Name:        "bar",
			},
			structs.TypedBag{
				KindVersion: kindVersion,
				Name:        "foo",
			},
		} {
			var out bool
			require.NoError(t, msgpackrpc.CallWithCodec(codec, "TypedBag.Apply", &structs.TypedBagRequest{
				Op:  structs.TypedBagUpsert,
				Bag: bag,
			}, &out))
			require.True(t, out)
		}
	}

	testutil.RunStep(t, "test the errNotChanged path", func(t *testing.T) {
		run(t, "completely-different-other")
	})
}

func TestTypedBag_List_ACLDeny(t *testing.T) {
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
	testrpc.WaitForTestAgent(t, s1.RPC, "dc1", testrpc.WithToken("root"))
	codec := rpcClient(t, s1)
	defer codec.Close()

	kindVersion := structs.KindVersion{
		Kind:    "foo",
		Version: "1",
	}

	rules := `
mesh = "write"
`
	id := createToken(t, codec, rules)

	// Create some dummy records to be looked up.
	state := s1.fsm.State()
	require.NoError(t, state.EnsureTypedBag(1, &structs.TypedBag{
		KindVersion: kindVersion,
		Name:        "foo",
	}))
	require.NoError(t, state.EnsureTypedBag(2, &structs.TypedBag{
		KindVersion: kindVersion,
		Name:        "bar",
	}))
	require.NoError(t, state.EnsureTypedBag(3, &structs.TypedBag{
		KindVersion: structs.KindVersion{
			Kind:    "bar",
			Version: "1",
		},
		Name: "baz",
	}))

	// This should fail since we don't have mesh permissions.
	args := structs.TypedBagQuery{
		Datacenter:  s1.config.Datacenter,
		KindVersion: kindVersion,
	}

	var out structs.TypedBagListResponse
	err := msgpackrpc.CallWithCodec(codec, "TypedBag.List", &args, &out)
	if !acl.IsErrPermissionDenied(err) {
		t.Fatalf("err: %v", err)
	}

	// do it again with the token
	args.QueryOptions.Token = id
	err = msgpackrpc.CallWithCodec(codec, "TypedBag.List", &args, &out)
	require.NoError(t, err)

	require.Len(t, out.TypedBags, 2)
	bag := out.TypedBags[0]
	require.Equal(t, kindVersion, bag.KindVersion)
}
