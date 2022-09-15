package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

var TypedBagSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"typed-bag", "apply"},
		Help: "Measures the time it takes to complete an update to the typed bag store.",
	},
}

type TypedBag struct {
	srv    *Server
	logger hclog.Logger
}

func (k *TypedBag) authCheck(entMeta *acl.EnterpriseMeta, token string) error {
	authz, err := k.srv.ResolveTokenAndDefaultMeta(token, entMeta, nil)
	if err != nil {
		return err
	}

	var authzContext acl.AuthorizerContext
	entMeta.FillAuthzContext(&authzContext)
	if err := authz.ToAllowAuthorizer().MeshWriteAllowed(&authzContext); err != nil {
		return err
	}

	return nil
}

func (k *TypedBag) Apply(args *structs.TypedBagRequest, reply *bool) error {
	if done, err := k.srv.ForwardRPC("TypedBag.Apply", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"typed-bag", "apply"}, time.Now())

	if err := k.authCheck(&args.Bag.EnterpriseMeta, args.Token); err != nil {
		return err
	}

	// Apply the update.
	resp, err := k.srv.raftApply(structs.TypedBagRequestType, args)
	if err != nil {
		return fmt.Errorf("raft apply failed: %w", err)
	}

	// Check if the return type is a bool.
	if respBool, ok := resp.(bool); ok {
		*reply = respBool
	}
	return nil
}

func (k *TypedBag) Get(args *structs.TypedBagQuery, reply *structs.TypedBagResponse) error {
	if done, err := k.srv.ForwardRPC("TypedBag.Get", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"typed-bag", "get"}, time.Now())

	if err := k.authCheck(&args.EnterpriseMeta, args.Token); err != nil {
		return err
	}

	return k.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, bag, err := state.TypedBag(ws, args.KindVersion, args.Name, args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.TypedBag = index, bag
			if bag == nil {
				return errNotFound
			}

			return nil
		})
}

func (k *TypedBag) List(args *structs.TypedBagQuery, reply *structs.TypedBagListResponse) error {
	if done, err := k.srv.ForwardRPC("TypedBag.List", args, reply); done {
		return err
	}

	defer metrics.MeasureSince([]string{"typed-bag", "list"}, time.Now())

	if err := k.authCheck(&args.EnterpriseMeta, args.Token); err != nil {
		return err
	}

	var (
		priorHash uint64
		ranOnce   bool
	)
	return k.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, bags, err := state.TypedBagsByKindVersion(ws, args.KindVersion)
			if err != nil {
				return err
			}

			reply.KindVersion = args.KindVersion
			reply.Index = index
			reply.TypedBags = bags

			// Generate a hash of the content driving this response. Use it to
			// determine if the response is identical to a prior wakeup.
			newHash, err := hashstructure_v2.Hash(bags, hashstructure_v2.FormatV2, nil)
			if err != nil {
				return fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
			}

			if ranOnce && priorHash == newHash {
				priorHash = newHash
				return errNotChanged
			} else {
				priorHash = newHash
				ranOnce = true
			}

			if len(reply.TypedBags) == 0 {
				return errNotFound
			}

			return nil
		})
}
