// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANserver.go KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rules

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/lack-io/vine/internal/auth/namespace"
	pb "github.com/lack-io/vine/proto/auth"
	"github.com/lack-io/vine/service/auth"
	"github.com/lack-io/vine/service/errors"
	"github.com/lack-io/vine/service/logger"
	"github.com/lack-io/vine/service/store"
	gostore "github.com/lack-io/vine/service/store"
)

const (
	storePrefixRules = "rules"
	joinKey          = "/"
)

var defaultRule = &auth.Rule{
	ID:     "default",
	Scope:  auth.ScopePublic,
	Access: auth.AccessGranted,
	Resource: &auth.Resource{
		Type:     "*",
		Name:     "*",
		Endpoint: "*",
	},
}

// Rules processes RPC calls
type Rules struct {
	Options auth.Options

	namespaces map[string]bool
	sync.Mutex
}

// Init the auth
func (r *Rules) Init(opts ...auth.Option) {
	for _, o := range opts {
		o(&r.Options)
	}
}

func (r *Rules) setupDefaultRules(ns string) {
	r.Lock()
	defer r.Unlock()

	// setup the namespace cache if not yet done
	if r.namespaces == nil {
		r.namespaces = make(map[string]bool)
	}

	// check to see if the default rule has already been verified
	if _, ok := r.namespaces[ns]; ok {
		return
	}

	// check to see if we need to create the default account
	key := strings.Join([]string{storePrefixRules, ns, ""}, joinKey)
	recs, err := store.DefaultStore.Read(key, gostore.ReadPrefix())
	if err != nil {
		return
	}

	// create the account if none exist in the namespace
	if len(recs) == 0 {
		rule := &pb.Rule{
			Id:     defaultRule.ID,
			Scope:  defaultRule.Scope,
			Access: pb.Access_GRANTED,
			Resource: &pb.Resource{
				Type:     defaultRule.Resource.Type,
				Name:     defaultRule.Resource.Name,
				Endpoint: defaultRule.Resource.Endpoint,
			},
		}

		if err := r.writeRule(rule, ns); err != nil {
			if logger.V(logger.WarnLevel, logger.DefaultLogger) {
				logger.Warnf("Error creating default rule: %v", err)
			}
		}
	}

	// set the namespace in the cache
	r.namespaces[ns] = true
}

// Create a rule giving a scope access to a resource
func (r *Rules) Create(ctx context.Context, req *pb.CreateRequest, rsp *pb.CreateResponse) error {
	// Validate the request
	if req.Rule == nil {
		return errors.BadRequest("auth.Rules.Create", "Rule missing")
	}
	if len(req.Rule.Id) == 0 {
		return errors.BadRequest("auth.Rules.Create", "ID missing")
	}
	if req.Rule.Resource == nil {
		return errors.BadRequest("auth.Rules.Create", "Resource missing")
	}
	if req.Rule.Access == pb.Access_UNKNOWN {
		return errors.BadRequest("auth.Rules.Create", "Access missing")
	}

	// set defaults
	if req.Options == nil {
		req.Options = &pb.Options{}
	}
	if len(req.Options.Namespace) == 0 {
		req.Options.Namespace = namespace.DefaultNamespace
	}

	// authorize the request
	if err := namespace.Authorize(ctx, req.Options.Namespace); err == namespace.ErrForbidden {
		return errors.Forbidden("auth.Rules.Create", err.Error())
	} else if err == namespace.ErrUnauthorized {
		return errors.Unauthorized("auth.Rules.Create", err.Error())
	} else if err != nil {
		return errors.InternalServerError("auth.Rules.Create", err.Error())
	}

	// write the rule to the store
	return r.writeRule(req.Rule, req.Options.Namespace)
}

// Delete a scope access to a resource
func (r *Rules) Delete(ctx context.Context, req *pb.DeleteRequest, rsp *pb.DeleteResponse) error {
	// Validate the request
	if len(req.Id) == 0 {
		return errors.BadRequest("auth.Rules.Delete", "ID missing")
	}

	// set defaults
	if req.Options == nil {
		req.Options = &pb.Options{}
	}
	if len(req.Options.Namespace) == 0 {
		req.Options.Namespace = namespace.DefaultNamespace
	}

	// authorize the request
	if err := namespace.Authorize(ctx, req.Options.Namespace); err == namespace.ErrForbidden {
		return errors.Forbidden("auth.Rules.Delete", err.Error())
	} else if err == namespace.ErrUnauthorized {
		return errors.Unauthorized("auth.Rules.Delete", err.Error())
	} else if err != nil {
		return errors.InternalServerError("auth.Rules.Delete", err.Error())
	}

	// Delete the rule
	key := strings.Join([]string{storePrefixRules, req.Options.Namespace, req.Id}, joinKey)
	err := store.DefaultStore.Delete(key)
	if err == gostore.ErrNotFound {
		return errors.BadRequest("auth.Rules.Delete", "Rule not found")
	} else if err != nil {
		return errors.InternalServerError("auth.Rules.Delete", "Unable to delete key from store: %v", err)
	}

	// Clear the namespace cache, since the rules for this namespace could now be empty
	r.Lock()
	delete(r.namespaces, req.Options.Namespace)
	r.Unlock()

	return nil
}

// List returns all the rules
func (r *Rules) List(ctx context.Context, req *pb.ListRequest, rsp *pb.ListResponse) error {
	// set defaults
	if req.Options == nil {
		req.Options = &pb.Options{}
	}
	if len(req.Options.Namespace) == 0 {
		req.Options.Namespace = namespace.DefaultNamespace
	}

	// authorize the request
	if err := namespace.Authorize(ctx, req.Options.Namespace); err == namespace.ErrForbidden {
		return errors.Forbidden("auth.Rules.List", err.Error())
	} else if err == namespace.ErrUnauthorized {
		return errors.Unauthorized("auth.Rules.List", err.Error())
	} else if err != nil {
		return errors.InternalServerError("auth.Rules.List", err.Error())
	}

	// setup the defaults incase none exist
	r.setupDefaultRules(req.Options.Namespace)

	// get the records from the store
	prefix := strings.Join([]string{storePrefixRules, req.Options.Namespace, ""}, joinKey)
	recs, err := store.DefaultStore.Read(prefix, gostore.ReadPrefix())
	if err != nil {
		return errors.InternalServerError("auth.Rules.List", "Unable to read from store: %v", err)
	}

	// unmarshal the records
	rsp.Rules = make([]*pb.Rule, 0, len(recs))
	for _, rec := range recs {
		var r *pb.Rule
		if err := json.Unmarshal(rec.Value, &r); err != nil {
			return errors.InternalServerError("auth.Rules.List", "Error to unmarshaling json: %v. Value: %v", err, string(rec.Value))
		}
		rsp.Rules = append(rsp.Rules, r)
	}

	return nil
}

// writeRule to the store
func (r *Rules) writeRule(rule *pb.Rule, ns string) error {
	key := strings.Join([]string{storePrefixRules, ns, rule.Id}, joinKey)
	if _, err := store.DefaultStore.Read(key); err == nil {
		return errors.BadRequest("auth.Rules.Create", "A rule with this ID already exists")
	}

	// Encode the rule
	bytes, err := json.Marshal(rule)
	if err != nil {
		return errors.InternalServerError("auth.Rules.Create", "Unable to marshal rule: %v", err)
	}

	// Write to the store
	if err := store.DefaultStore.Write(&gostore.Record{Key: key, Value: bytes}); err != nil {
		return errors.InternalServerError("auth.Rules.Create", "Unable to write to the store: %v", err)
	}

	return nil
}
