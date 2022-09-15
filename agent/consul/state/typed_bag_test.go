package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func TestStore_TypedBag(t *testing.T) {
	s := testConfigStateStore(t)

	expected := &structs.TypedBag{
		KindVersion: structs.KindVersion{
			Kind:    "foo",
			Version: "1",
		},
		Name: "bar",
		Data: []byte("baz"),
	}

	// Create
	require.NoError(t, s.EnsureTypedBag(0, expected))

	idx, config, err := s.TypedBag(nil, expected.KindVersion, "bar", acl.EnterpriseMeta{})
	require.NoError(t, err)
	require.Equal(t, uint64(0), idx)
	require.Equal(t, expected, config)

	// Update
	updated := &structs.TypedBag{
		KindVersion: structs.KindVersion{
			Kind:    "foo",
			Version: "1",
		},
		Name: "bar",
		Data: []byte("zoiks"),
	}

	require.NoError(t, s.EnsureTypedBag(1, updated))

	idx, config, err = s.TypedBag(nil, updated.KindVersion, "bar", acl.EnterpriseMeta{})
	require.NoError(t, err)
	require.Equal(t, uint64(1), idx)
	require.Equal(t, updated, config)

	// Delete
	require.NoError(t, s.DeleteTypedBag(2, updated.KindVersion, "bar", acl.EnterpriseMeta{}))

	idx, config, err = s.TypedBag(nil, updated.KindVersion, "bar", acl.EnterpriseMeta{})
	require.NoError(t, err)
	require.Equal(t, uint64(2), idx)
	require.Nil(t, config)

	// Set up a watch.
	bag := &structs.TypedBag{
		KindVersion: structs.KindVersion{
			Kind:    "bar",
			Version: "1",
		},
		Name: "bar",
		Data: []byte("test"),
	}
	require.NoError(t, s.EnsureTypedBag(3, bag))

	ws := memdb.NewWatchSet()
	_, _, err = s.TypedBag(ws, bag.KindVersion, "bar", acl.EnterpriseMeta{})
	require.NoError(t, err)

	// Make an unrelated modification and make sure the watch doesn't fire.
	require.NoError(t, s.EnsureTypedBag(4, updated))
	require.False(t, watchFired(ws))

	// Update the watched config and make sure it fires.
	bag.Data = []byte("test2")
	require.NoError(t, s.EnsureTypedBag(5, bag))
	require.True(t, watchFired(ws))
}
