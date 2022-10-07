package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/stretchr/testify/require"
)

func TestBasicController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reconciler := newTestReconciler(false)

	publisher := stream.NewEventPublisher(1 * time.Millisecond)
	go publisher.Run(ctx)

	// get the store through the FSM since the publisher handlers get registered through it
	store := fsm.NewFromDeps(fsm.Deps{
		Logger: hclog.New(nil),
		NewStateStore: func() *state.Store {
			return state.NewStateStoreWithEventPublisher(nil, publisher)
		},
		Publisher: publisher,
	}).State()

	for i := 0; i < 200; i++ {
		entryIndex := uint64(i + 1)
		name := fmt.Sprintf("foo-%d", i)
		require.NoError(t, store.EnsureConfigEntry(entryIndex, &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: name,
		}))
	}

	go New(publisher, reconciler).Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicIngressGateway,
		Subject: stream.SubjectWildcard,
	}).WithWorkers(10).Start(ctx)

	received := []string{}
LOOP:
	for {
		select {
		case request := <-reconciler.received:
			require.Equal(t, structs.IngressGateway, request.Kind)
			received = append(received, request.Name)
			if len(received) == 200 {
				break LOOP
			}
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

func TestBasicController_Transform(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	reconciler := newTestReconciler(false)

	publisher := stream.NewEventPublisher(0)
	go publisher.Run(ctx)

	// get the store through the FSM since the publisher handlers get registered through it
	store := fsm.NewFromDeps(fsm.Deps{
		Logger: hclog.New(nil),
		NewStateStore: func() *state.Store {
			return state.NewStateStoreWithEventPublisher(nil, publisher)
		},
		Publisher: publisher,
	}).State()

	go New(publisher, reconciler).Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicIngressGateway,
		Subject: stream.SubjectWildcard,
	}, func(entry structs.ConfigEntry) []Request {
		return []Request{{
			Kind: "foo",
			Name: "bar",
		}}
	}).Start(ctx)

	require.NoError(t, store.EnsureConfigEntry(1, &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "test",
	}))

	select {
	case request := <-reconciler.received:
		require.Equal(t, "foo", request.Kind)
		require.Equal(t, "bar", request.Name)
	case <-ctx.Done():
		t.Fatal("stopped reconciler before event received")
	}
}

func TestBasicController_Retry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	reconciler := newTestReconciler(true)
	defer reconciler.stop()

	publisher := stream.NewEventPublisher(0)
	go publisher.Run(ctx)

	// get the store through the FSM since the publisher handlers get registered through it
	store := fsm.NewFromDeps(fsm.Deps{
		Logger: hclog.New(nil),
		NewStateStore: func() *state.Store {
			return state.NewStateStoreWithEventPublisher(nil, publisher)
		},
		Publisher: publisher,
	}).State()

	queueInitialized := make(chan *countingWorkQueue)
	controller := New(publisher, reconciler).Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicIngressGateway,
		Subject: stream.SubjectWildcard,
	}).WithWorkers(-1).WithBackoff(1*time.Millisecond, 1*time.Millisecond)
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
			require.Equal(t, structs.IngressGateway, req.Kind)
			require.Equal(t, name, req.Name)
			return true
		case <-time.After(10 * time.Millisecond):
			return false
		}
	}

	// check to make sure we are called once
	queue.reset()
	require.NoError(t, store.EnsureConfigEntry(1, &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
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
	require.NoError(t, store.EnsureConfigEntry(2, &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
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
	require.NoError(t, store.EnsureConfigEntry(3, &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
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

func TestBasicController_Triggers(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reconciler := newTestReconciler(true)

	publisher := stream.NewEventPublisher(0)
	go publisher.Run(ctx)

	controller := New(publisher, reconciler)

	go func() {
		require.NoError(t, controller.Start(ctx))
	}()

	ensureCalled := func(request chan Request, name string) bool {
		select {
		case req := <-request:
			require.Equal(t, structs.IngressGateway, req.Kind)
			require.Equal(t, name, req.Name)
			return true
		case <-time.After(10 * time.Millisecond):
			return false
		}
	}

	request := Request{
		Kind: structs.IngressGateway,
		Name: "foo-1",
	}

	trigger := make(chan struct{}, 3)
	controller.AddTrigger(request, func(ctx context.Context) error {
		select {
		case <-trigger:
			return nil
		case <-ctx.Done():
			return nil
		}
	})
	require.False(t, ensureCalled(reconciler.received, "foo-1"))
	trigger <- struct{}{}
	reconciler.stepFor(10 * time.Millisecond)
	require.True(t, ensureCalled(reconciler.received, "foo-1"))
	require.False(t, ensureCalled(reconciler.received, "foo-1"))
	trigger <- struct{}{}
	reconciler.stepFor(10 * time.Millisecond)
	require.True(t, ensureCalled(reconciler.received, "foo-1"))

	// check with the overwritten trigger
	triggerTwo := make(chan struct{}, 2)
	controller.AddTrigger(request, func(ctx context.Context) error {
		select {
		case <-triggerTwo:
			return nil
		case <-ctx.Done():
			return nil
		}
	})
	trigger <- struct{}{}
	reconciler.stepFor(10 * time.Millisecond)
	require.False(t, ensureCalled(reconciler.received, "foo-1"))
	triggerTwo <- struct{}{}
	reconciler.stepFor(10 * time.Millisecond)
	require.True(t, ensureCalled(reconciler.received, "foo-1"))

	// remove the trigger and make sure we're not called again
	controller.RemoveTrigger(request)
	triggerTwo <- struct{}{}
	reconciler.stepFor(10 * time.Millisecond)
	require.False(t, ensureCalled(reconciler.received, "foo-1"))
}

func TestDiscoveryChainController(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reconciler := newTestReconciler(false)

	publisher := stream.NewEventPublisher(1 * time.Millisecond)
	go publisher.Run(ctx)

	// get the store through the FSM since the publisher handlers get registered through it
	store := fsm.NewFromDeps(fsm.Deps{
		Logger: hclog.New(nil),
		NewStateStore: func() *state.Store {
			return state.NewStateStoreWithEventPublisher(nil, publisher)
		},
		Publisher: publisher,
	}).State()

	controller := New(publisher, reconciler)
	go controller.Subscribe(&stream.SubscribeRequest{
		Topic:   state.EventTopicIngressGateway,
		Subject: stream.SubjectWildcard,
	}).WithWorkers(10).Start(ctx)

	request := Request{
		Kind: structs.IngressGateway,
		Name: "foo-1",
	}
	trigger := func(ctx context.Context) error {
		ws := memdb.NewWatchSet()
		ws.Add(store.AbandonCh())
		_, _, err := store.ReadDiscoveryChainConfigEntries(ws, "foo-2", nil, nil)
		if err != nil {
			return err
		}
		return ws.WatchCtx(ctx)
	}
	controller.AddTrigger(request, trigger)

	ensureCalled := func(request chan Request, name string) bool {
		select {
		case req := <-request:
			require.Equal(t, structs.IngressGateway, req.Kind)
			require.Equal(t, name, req.Name)
			return true
		case <-time.After(10 * time.Millisecond):
			return false
		}
	}

	require.NoError(t, store.EnsureConfigEntry(1, &structs.IngressGatewayConfigEntry{
		Kind: structs.IngressGateway,
		Name: "foo-1",
	}))
	require.True(t, ensureCalled(reconciler.received, "foo-1"))

	// create something that changes in its upstream discovery chain and ensure that we've
	// fired the reconciler
	require.False(t, ensureCalled(reconciler.received, "foo-1"))
	require.NoError(t, store.EnsureConfigEntry(1, &structs.ServiceResolverConfigEntry{
		Kind: structs.ServiceResolver,
		Name: "foo-2",
	}))
	require.True(t, ensureCalled(reconciler.received, "foo-1"))
}
