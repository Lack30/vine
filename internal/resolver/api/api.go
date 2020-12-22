// Package vine provides a vine rpc resolver which prefixes a namespace
package api

import (
	"net/http"

	"github.com/lack-io/vine/internal/api/resolver"
	"github.com/lack-io/vine/service/registry"
)

// default resolver for legacy purposes
// it uses proxy routing to resolve names
// /foo becomes namespace.foo
// /v1/foo becomes namespace.v1.foo
type Resolver struct {
	opts resolver.Options
}

func (r *Resolver) Resolve(req *http.Request, opts ...resolver.ResolveOption) (*resolver.Endpoint, error) {
	options := resolver.NewResolveOptions(opts...)

	var name, method string

	switch r.opts.Handler {
	// internal handlers
	case "meta", "api", "rpc", "vine":
		name, method = apiRoute(req.URL.Path)
	default:
		method = req.Method
		name = proxyRoute(req.URL.Path)
	}

	// append the service prefix, e.g. foo.api
	if len(r.opts.ServicePrefix) > 0 {
		name = r.opts.ServicePrefix + "." + name
	}

	// check for the namespace in the request header, this can be set by the client or injected
	// by the auth wrapper if an auth token was provided. The headr takes priority over any domain
	// passed as a default
	domain := options.Domain
	if dom := req.Header.Get("Vine-Namespace"); len(dom) > 0 && dom != domain {
		domain = dom
	} else if len(domain) == 0 {
		domain = registry.DefaultDomain
	}

	return &resolver.Endpoint{
		Name:   name,
		Domain: domain,
		Method: method,
	}, nil
}

func (r *Resolver) String() string {
	return "vine"
}

// NewResolver creates a new vine resolver
func NewResolver(opts ...resolver.Option) resolver.Resolver {
	return &Resolver{
		opts: resolver.NewOptions(opts...),
	}
}
