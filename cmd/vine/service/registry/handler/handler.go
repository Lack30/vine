// Copyright 2020 The vine Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"context"
	"strings"
	"time"

	"github.com/lack-io/vine/proto/errors"
	pb "github.com/lack-io/vine/proto/registry"
	"github.com/lack-io/vine/service"
	"github.com/lack-io/vine/service/auth"
	log "github.com/lack-io/vine/service/logger"
	"github.com/lack-io/vine/service/registry"
	"github.com/lack-io/vine/service/registry/grpc"
	"github.com/lack-io/vine/util/namespace"
)

type Registry struct {
	// service id
	Id string
	// the publisher
	Publisher service.Event
	// internal registry
	Registry registry.Registry
	// auth to verify clients
	Auth auth.Auth
}

func ActionToEventType(action string) registry.EventType {
	switch action {
	case "create":
		return registry.Create
	case "delete":
		return registry.Delete
	default:
		return registry.Update
	}
}

func (r *Registry) publishEvent(action string, service *pb.Service) error {
	// TODO: timestamp should be read from received event
	// Right now registry.Result does not contain timestamp
	event := &pb.Event{
		Id:        r.Id,
		Type:      pb.EventType(ActionToEventType(action)),
		Timestamp: time.Now().UnixNano(),
		Service:   service,
	}

	log.Debugf("publishing event %s for action %s", event.Id, action)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.Publisher.Publish(ctx, event)
}

// GetService from the registry with the name requested
func (r *Registry) GetService(ctx context.Context, req *pb.GetRequest, rsp *pb.GetResponse) error {
	// get the services in the default namespace
	services, err := r.Registry.GetService(req.Service)
	if err == registry.ErrNotFound {
		return errors.NotFound("go.vine.registry", err.Error())
	} else if err != nil {
		return errors.InternalServerError("go.vine.registry", err.Error())
	}

	// get the services in the requested namespace, e.g. the "foo" namespace. name
	// includes the namespace as the prefix, e.g. 'foo/go.vine.service.bar'
	if namespace.FromContext(ctx) != namespace.DefaultNamespace {
		name := namespace.FromContext(ctx) + nameSeperator + req.Service
		srvs, err := r.Registry.GetService(name)
		if err != nil {
			return errors.InternalServerError("go.vine.registry", err.Error())
		}
		services = append(services, srvs...)
	}

	for _, srv := range services {
		rsp.Services = append(rsp.Services, grpc.ToProto(withoutNamespace(*srv)))
	}
	return nil
}

// Register a service
func (r *Registry) Register(ctx context.Context, req *pb.Service, rsp *pb.EmptyResponse) error {
	var regOpts []registry.RegisterOption
	if req.Options != nil {
		ttl := time.Duration(req.Options.Ttl) * time.Second
		regOpts = append(regOpts, registry.RegisterTTL(ttl))
	}

	service := grpc.ToService(withNamespace(*req, namespace.FromContext(ctx)))
	if err := r.Registry.Register(service, regOpts...); err != nil {
		return errors.InternalServerError("go.vine.registry", err.Error())
	}

	// publish the event
	go r.publishEvent("create", req)

	return nil
}

// Deregister a service
func (r *Registry) Deregister(ctx context.Context, req *pb.Service, rsp *pb.EmptyResponse) error {
	service := grpc.ToService(withNamespace(*req, namespace.FromContext(ctx)))
	if err := r.Registry.Deregister(service); err != nil {
		return errors.InternalServerError("go.vine.registry", err.Error())
	}

	// publish the event
	go r.publishEvent("delete", req)

	return nil
}

// ListServices returns all the services
func (r *Registry) ListServices(ctx context.Context, req *pb.ListRequest, rsp *pb.ListResponse) error {
	services, err := r.Registry.ListServices()
	if err != nil {
		return errors.InternalServerError("go.vine.registry", err.Error())
	}

	for _, srv := range services {
		// check to see if the service belongs to the defaut namespace
		// or the contexts namespace. TODO: think about adding a prefix
		//argument to ListServices
		if !canReadService(ctx, srv) {
			continue
		}

		rsp.Services = append(rsp.Services, grpc.ToProto(withoutNamespace(*srv)))
	}

	return nil
}

// Watch a service for changes
func (r *Registry) Watch(ctx context.Context, req *pb.WatchRequest, rsp pb.Registry_WatchStream) error {
	watcher, err := r.Registry.Watch(registry.WatchService(req.Service))
	if err != nil {
		return errors.InternalServerError("go.vine.registry", err.Error())
	}

	for {
		next, err := watcher.Next()
		if err != nil {
			return errors.InternalServerError("go.vine.registry", err.Error())
		}
		if !canReadService(ctx, next.Service) {
			continue
		}

		err = rsp.Send(&pb.Result{
			Action:  next.Action,
			Service: grpc.ToProto(withoutNamespace(*next.Service)),
		})
		if err != nil {
			return errors.InternalServerError("go.vine.registry", err.Error())
		}
	}
}

// canReadService is a helper function which returns a boolean indicating
// if a context can read a service.
func canReadService(ctx context.Context, srv *registry.Service) bool {
	// check if the service has no prefix which means it was written
	// directly to the store and is therefore assumed to be part of
	// the default namespace
	if len(strings.Split(srv.Name, nameSeperator)) == 1 {
		return true
	}

	// the service belongs to the contexts namespace
	if strings.HasPrefix(srv.Name, namespace.FromContext(ctx)+nameSeperator) {
		return true
	}

	return false
}

// nameSeperator is the string which is used as a seperator when joining
// namespace to the service name
const nameSeperator = "/"

// withoutNamespace returns the service with the namespace stripped from
// the name, e.g. 'bar/go.vine.service.foo' => 'go.vine.service.foo'.
func withoutNamespace(srv registry.Service) *registry.Service {
	comps := strings.Split(srv.Name, nameSeperator)
	srv.Name = comps[len(comps)-1]
	return &srv
}

// withNamespace returns the service with the namespace prefixed to the
// name, e.g. 'go.vine.service.foo' => 'bar/go.vine.service.foo'
func withNamespace(srv pb.Service, ns string) *pb.Service {
	// if the namespace is the default, don't append anything since this
	// means users not leveraging multi-tenancy won't experience any changes
	if ns == namespace.DefaultNamespace {
		return &srv
	}

	srv.Name = strings.Join([]string{ns, srv.Name}, nameSeperator)
	return &srv
}
