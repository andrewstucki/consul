package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestBasicController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	store := state.NewStateStoreWithEventPublisher(nil, stream.NoOpEventPublisher{})

	reconciler := newTestReconciler(false)
	go New(store, reconciler).Watch(structs.ServiceDefaults, nil).Start(ctx)

	for i := 0; i < 200; i++ {
		entryIndex := uint64(i + 1)
		name := fmt.Sprintf("foo-%d", i)
		require.NoError(t, store.EnsureConfigEntry(entryIndex, &structs.ServiceConfigEntry{
			Kind: structs.ServiceDefaults,
			Name: name,
		}))
	}

	received := []string{}
LOOP:
	for {
		select {
		case request := <-reconciler.received:
			require.Equal(t, structs.ServiceDefaults, request.Kind)
			received = append(received, request.Name)
		case <-ctx.Done():
			break LOOP
		}
	}

	// since we only modified each entry once, we should have exactly 200 reconcliation calls
	require.Len(t, received, 200)
	for i := 0; i < 200; i++ {
		require.Contains(t, received, fmt.Sprintf("foo-%d", i))
	}
}

func TestBasicController_Retry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	store := state.NewStateStoreWithEventPublisher(nil, stream.NoOpEventPublisher{})

	reconciler := newTestReconciler(true)
	defer reconciler.stop()

	queueInitialized := make(chan *countingWorkQueue)
	controller := New(store, reconciler).Watch(structs.ServiceDefaults, nil).WithBackoff(1*time.Millisecond, 1*time.Millisecond)
	go controller.WithQueueFactory(func(ctx context.Context, baseBackoff, maxBackoff time.Duration) WorkQueue {
		queue := newCountingWorkQueue(RunWorkQueue(ctx, baseBackoff, maxBackoff))
		queueInitialized <- queue
		return queue
	}).Start(ctx)

	queue := <-queueInitialized

	ensureCalled := func(request chan Request, name string) bool {
		// give a short window for our counters to update
		defer time.Sleep(10 * time.Millisecond)
		select {
		case req := <-request:
			require.Equal(t, structs.ServiceDefaults, req.Kind)
			require.Equal(t, name, req.Name)
			return true
		case <-time.After(10 * time.Millisecond):
			return false
		}
	}

	// check to make sure we are called once
	queue.reset()
	require.NoError(t, store.EnsureConfigEntry(1, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo-1",
	}))
	require.False(t, ensureCalled(reconciler.received, "foo-1"))
	require.EqualValues(t, 0, queue.dones())
	require.EqualValues(t, 0, queue.requeues())
	reconciler.step()
	require.True(t, ensureCalled(reconciler.received, "foo-1"))
	require.EqualValues(t, 1, queue.dones())
	require.EqualValues(t, 0, queue.requeues())

	// check that we requeue when an arbitrary error occurs
	queue.reset()
	reconciler.setResponse(errors.New("error"))
	require.NoError(t, store.EnsureConfigEntry(2, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo-2",
	}))
	require.False(t, ensureCalled(reconciler.received, "foo-2"))
	require.EqualValues(t, 0, queue.dones())
	require.EqualValues(t, 0, queue.requeues())
	require.EqualValues(t, 0, queue.addRateLimiteds())
	reconciler.step()
	// check we're processed the first time and re-queued
	require.True(t, ensureCalled(reconciler.received, "foo-2"))
	require.EqualValues(t, 1, queue.dones())
	require.EqualValues(t, 1, queue.requeues())
	require.EqualValues(t, 1, queue.addRateLimiteds())
	// now make sure we succeed
	reconciler.setResponse(nil)
	reconciler.step()
	require.True(t, ensureCalled(reconciler.received, "foo-2"))
	require.EqualValues(t, 2, queue.dones())
	require.EqualValues(t, 1, queue.requeues())
	require.EqualValues(t, 1, queue.addRateLimiteds())

	// check that we requeue at a given rate when using a RequeueAfterError
	queue.reset()
	reconciler.setResponse(RequeueNow())
	require.NoError(t, store.EnsureConfigEntry(3, &structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: "foo-3",
	}))
	require.False(t, ensureCalled(reconciler.received, "foo-3"))
	require.EqualValues(t, 0, queue.dones())
	require.EqualValues(t, 0, queue.requeues())
	require.EqualValues(t, 0, queue.addRateLimiteds())
	reconciler.step()
	// check we're processed the first time and re-queued
	require.True(t, ensureCalled(reconciler.received, "foo-3"))
	require.EqualValues(t, 1, queue.dones())
	require.EqualValues(t, 1, queue.requeues())
	require.EqualValues(t, 1, queue.addAfters())
	// now make sure we succeed
	reconciler.setResponse(nil)
	reconciler.step()
	require.True(t, ensureCalled(reconciler.received, "foo-3"))
	require.EqualValues(t, 2, queue.dones())
	require.EqualValues(t, 1, queue.requeues())
	require.EqualValues(t, 1, queue.addAfters())

}
