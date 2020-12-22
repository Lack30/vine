// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"

	authns "github.com/lack-io/vine/internal/auth/namespace"
	"github.com/lack-io/vine/internal/namespace"
	pb "github.com/lack-io/vine/proto/network"
	pbRtr "github.com/lack-io/vine/proto/router"
	"github.com/lack-io/vine/service/errors"
	log "github.com/lack-io/vine/service/logger"
	"github.com/lack-io/vine/service/network"
	"github.com/lack-io/vine/service/network/mucp"
	"github.com/lack-io/vine/service/network/util"
	"github.com/lack-io/vine/service/router"
)

// Network implements network handler
type Network struct {
	Network network.Network
}

func flatten(n network.Node, visited map[string]bool) []network.Node {
	// if node is nil runaway
	if n == nil {
		return nil
	}

	// set visisted
	if visited == nil {
		visited = make(map[string]bool)
	}

	// create new list of nodes
	//nolint:prealloc
	var nodes []network.Node

	// check if already visited
	if !visited[n.Id()] {
		// append the current node
		nodes = append(nodes, n)
	}

	// set to visited
	visited[n.Id()] = true

	// visit the list of peers
	for _, node := range n.Peers() {
		nodes = append(nodes, flatten(node, visited)...)
	}

	return nodes
}

func (n *Network) Connect(ctx context.Context, req *pb.ConnectRequest, resp *pb.ConnectResponse) error {
	if len(req.Nodes) == 0 {
		return nil
	}

	// get list of existing nodes
	nodes := n.Network.Options().Nodes

	// generate a node map
	nodeMap := make(map[string]bool)

	for _, node := range nodes {
		nodeMap[node] = true
	}

	for _, node := range req.Nodes {
		// TODO: we may have been provided a network only
		// so process anad resolve node.Network
		if len(node.Address) == 0 {
			continue
		}

		// already exists
		if _, ok := nodeMap[node.Address]; ok {
			continue
		}

		nodeMap[node.Address] = true
		nodes = append(nodes, node.Address)
	}

	log.Infof("Network.Connect setting peers: %v", nodes)

	// reinitialise the peers
	n.Network.Init(
		network.Nodes(nodes...),
	)

	// call the connect method
	n.Network.Connect()

	return nil
}

// Nodes returns the list of nodes
func (n *Network) Nodes(ctx context.Context, req *pb.NodesRequest, resp *pb.NodesResponse) error {
	// root node
	nodes := map[string]network.Node{}

	// get peers encoded into protobuf
	peers := flatten(n.Network, nil)

	// walk all the peers
	for _, peer := range peers {
		if peer == nil {
			continue
		}
		if _, ok := nodes[peer.Id()]; ok {
			continue
		}

		// add to visited list
		nodes[n.Network.Id()] = peer

		resp.Nodes = append(resp.Nodes, &pb.Node{
			Id:      peer.Id(),
			Address: peer.Address(),
		})
	}

	return nil
}

// Graph returns the network graph from this root node
func (n *Network) Graph(ctx context.Context, req *pb.GraphRequest, resp *pb.GraphResponse) error {
	depth := uint(req.Depth)
	if depth <= 0 || depth > mucp.MaxDepth {
		depth = mucp.MaxDepth
	}

	// get peers encoded into protobuf
	peers := util.PeersToProto(n.Network, depth)

	// set the root node
	resp.Root = peers

	return nil
}

// Routes returns a list of routing table routes
func (n *Network) Routes(ctx context.Context, req *pb.RoutesRequest, resp *pb.RoutesResponse) error {
	// default the network to the current users namespace
	if req.Query == nil {
		req.Query = &pb.Query{}
	}
	if len(req.Query.Network) == 0 {
		req.Query.Network = namespace.FromContext(ctx)
	}

	// authorize the request
	if err := authns.Authorize(ctx, req.Query.Network); err == authns.ErrForbidden {
		return errors.Forbidden("network.Network.Routes", err.Error())
	} else if err == authns.ErrUnauthorized {
		return errors.Unauthorized("network.Network.Routes", err.Error())
	} else if err != nil {
		return errors.InternalServerError("network.Network.Routes", err.Error())
	}

	// build query
	var qOpts []router.LookupOption
	if len(req.Query.Address) > 0 {
		qOpts = append(qOpts, router.LookupAddress(req.Query.Address))
	}
	if len(req.Query.Gateway) > 0 {
		qOpts = append(qOpts, router.LookupGateway(req.Query.Gateway))
	}
	if len(req.Query.Router) > 0 {
		qOpts = append(qOpts, router.LookupRouter(req.Query.Router))
	}

	// for users in the default namespace, allow access to all namespaces
	if req.Query.Network != namespace.DefaultNamespace {
		qOpts = append(qOpts, router.LookupNetwork(req.Query.Network))
	}

	var routes []router.Route
	var err error

	// if a service is specified to a router Lookup
	if len(req.Query.Service) > 0 {
		routes, err = n.Network.Options().Router.Lookup(req.Query.Service, qOpts...)
	} else {
		// otherwise list and filter
		routes, err := n.Network.Options().Router.Table().Read()
		if err == nil {
			// filter the routes
			routes = router.Filter(routes, router.NewLookup(qOpts...))
		}
	}

	if err != nil {
		return errors.InternalServerError("network.Network.Routes", "failed to list routes: %s", err)
	}

	respRoutes := make([]*pbRtr.Route, 0, len(routes))

	for _, route := range routes {
		respRoute := &pbRtr.Route{
			Service: route.Service,
			Address: route.Address,
			Gateway: route.Gateway,
			Network: route.Network,
			Router:  route.Router,
			Link:    route.Link,
			Metric:  int64(route.Metric),
		}
		respRoutes = append(respRoutes, respRoute)
	}

	resp.Routes = respRoutes

	return nil
}

// Services returns a list of services based on the routing table
func (n *Network) Services(ctx context.Context, req *pb.ServicesRequest, resp *pb.ServicesResponse) error {
	// authorize the request. only accounts issued by vine (root accounts) can access this endpoint
	if err := authns.Authorize(ctx, namespace.DefaultNamespace); err == authns.ErrForbidden {
		return errors.Forbidden("network.Network.Services", err.Error())
	} else if err == authns.ErrUnauthorized {
		return errors.Unauthorized("network.Network.Services", err.Error())
	} else if err != nil {
		return errors.InternalServerError("network.Network.Services", err.Error())
	}

	routes, err := n.Network.Options().Router.Table().Read()
	if err != nil {
		return errors.InternalServerError("network.Network.Services", "failed to list services: %s", err)
	}

	services := make(map[string]bool)

	for _, route := range routes {
		if route.Service == "*" {
			continue
		}

		if _, ok := services[route.Service]; ok {
			continue
		}
		services[route.Service] = true
		resp.Services = append(resp.Services, route.Service)
	}

	return nil
}
