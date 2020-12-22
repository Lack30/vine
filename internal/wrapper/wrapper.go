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

package wrapper

import (
	"context"
	"reflect"
	"strings"
	"time"

	inauth "github.com/lack-io/vine/internal/auth"
	"github.com/lack-io/vine/internal/auth/namespace"
	"github.com/lack-io/vine/internal/debug/trace"
	"github.com/lack-io/vine/service/auth"
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/client/cache"
	"github.com/lack-io/vine/service/context/metadata"
	"github.com/lack-io/vine/service/debug"
	"github.com/lack-io/vine/service/errors"
	"github.com/lack-io/vine/service/logger"
	"github.com/lack-io/vine/service/metrics"
	"github.com/lack-io/vine/service/server"
)

type authWrapper struct {
	client.Client
}

func (a *authWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	ctx = a.wrapContext(ctx, opts...)
	return a.Client.Call(ctx, req, rsp, opts...)
}

func (a *authWrapper) Stream(ctx context.Context, req client.Request, opts ...client.CallOption) (client.Stream, error) {
	ctx = a.wrapContext(ctx, opts...)
	return a.Client.Stream(ctx, req, opts...)
}

func (a *authWrapper) wrapContext(ctx context.Context, opts ...client.CallOption) context.Context {
	// parse the options
	var options client.CallOptions
	for _, o := range opts {
		o(&options)
	}

	// set the namespace header if it has not been set (e.g. on a service to service request)
	authOpts := auth.DefaultAuth.Options()
	if _, ok := metadata.Get(ctx, "Vine-Namespace"); !ok {
		ctx = metadata.Set(ctx, "Vine-Namespace", authOpts.Issuer)
	}

	// We dont't override the header unless the AuthToken option has been specified
	if !options.AuthToken {
		return ctx
	}

	// check to see if we have a valid access token
	if authOpts.Token != nil && !authOpts.Token.Expired() {
		ctx = metadata.Set(ctx, "Authorization", inauth.BearerScheme+authOpts.Token.AccessToken)
		return ctx
	}

	// call without an auth token
	return ctx
}

// AuthClient wraps requests with the auth header
func AuthClient(c client.Client) client.Client {
	return &authWrapper{c}
}

// AuthHandler wraps a server handler to perform auth
func AuthHandler() server.HandlerWrapper {
	return func(h server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			// Extract the token if the header is present. We will inspect the token regardless of if it's
			// present or not since noop auth will return a blank account upon Inspecting a blank token.
			var token string
			if header, ok := metadata.Get(ctx, "Authorization"); ok {
				// Ensure the correct scheme is being used
				if !strings.HasPrefix(header, inauth.BearerScheme) {
					return errors.Unauthorized(req.Service(), "invalid authorization header. expected Bearer schema")
				}

				// Strip the bearer scheme prefix
				token = strings.TrimPrefix(header, inauth.BearerScheme)
			}

			// Determine the namespace
			ns := auth.DefaultAuth.Options().Issuer

			var acc *auth.Account
			if a, err := auth.Inspect(token); err == nil && a.Issuer == ns {
				// We only use accounts issued by the same namespace as the service when verifying against
				// the rule set.
				ctx = auth.ContextWithAccount(ctx, a)
				acc = a
			} else if err == nil && ns == namespace.DefaultNamespace {
				// for the default domain, we want to inject the account into the context so that the
				// server can access it (since it's designed for multi-tenancy), however we don't want to
				// use it when verifying against the auth rules, since this will allow any user access to the
				// services running in the vine namespace
				ctx = auth.ContextWithAccount(ctx, a)
			}

			// ensure only accounts with the correct namespace can access this namespace,
			// since the auth package will verify access below, and some endpoints could
			// be public, we allow nil accounts access using the namespace.Public option.
			err := namespace.Authorize(ctx, ns, namespace.Public(ns))
			if err == namespace.ErrForbidden {
				return errors.Forbidden(req.Service(), err.Error())
			} else if err != nil {
				return errors.InternalServerError(req.Service(), err.Error())
			}

			// construct the resource
			res := &auth.Resource{
				Type:     "service",
				Name:     req.Service(),
				Endpoint: req.Endpoint(),
			}

			// Verify the caller has access to the resource.
			err = auth.Verify(acc, res, auth.VerifyNamespace(ns))
			if err == auth.ErrForbidden && acc != nil {
				return errors.Forbidden(req.Service(), "Forbidden call made to %v:%v by %v", req.Service(), req.Endpoint(), acc.ID)
			} else if err == auth.ErrForbidden {
				return errors.Unauthorized(req.Service(), "Unauthorized call made to %v:%v", req.Service(), req.Endpoint())
			} else if err != nil {
				return errors.InternalServerError(req.Service(), "Error authorizing request: %v", err)
			}

			// The user is authorised, allow the call
			return h(ctx, req, rsp)
		}
	}
}

type fromServiceWrapper struct {
	client.Client
}

var (
	HeaderPrefix = "Vine-"
)

func (f *fromServiceWrapper) setHeaders(ctx context.Context) context.Context {
	return metadata.MergeContext(ctx, metadata.Metadata{
		HeaderPrefix + "From-Service": server.DefaultServer.Options().Name,
	}, false)
}

func (f *fromServiceWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	ctx = f.setHeaders(ctx)
	return f.Client.Call(ctx, req, rsp, opts...)
}

func (f *fromServiceWrapper) Stream(ctx context.Context, req client.Request, opts ...client.CallOption) (client.Stream, error) {
	ctx = f.setHeaders(ctx)
	return f.Client.Stream(ctx, req, opts...)
}

func (f *fromServiceWrapper) Publish(ctx context.Context, p client.Message, opts ...client.PublishOption) error {
	ctx = f.setHeaders(ctx)
	return f.Client.Publish(ctx, p, opts...)
}

// FromService wraps a client to inject service and auth metadata
func FromService(c client.Client) client.Client {
	return &fromServiceWrapper{c}
}

type logWrapper struct {
	client.Client
}

func (l *logWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	logger.Debugf("Calling service %s endpoint %s", req.Service(), req.Endpoint())
	return l.Client.Call(ctx, req, rsp, opts...)
}

func LogClient(c client.Client) client.Client {
	return &logWrapper{c}
}

func LogHandler() server.HandlerWrapper {
	// return a handler wrapper
	return func(h server.HandlerFunc) server.HandlerFunc {
		// return a function that returns a function
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			logger.Debugf("Serving request for service %s endpoint %s", req.Service(), req.Endpoint())
			return h(ctx, req, rsp)
		}
	}
}

// HandlerStats wraps a server handler to generate request/error stats
func HandlerStats() server.HandlerWrapper {
	// return a handler wrapper
	return func(h server.HandlerFunc) server.HandlerFunc {
		// return a function that returns a function
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			// execute the handler
			err := h(ctx, req, rsp)
			// record the stats
			debug.DefaultStats.Record(err)
			// return the error
			return err
		}
	}
}

type traceWrapper struct {
	client.Client
}

func (c *traceWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	newCtx, s := debug.DefaultTracer.Start(ctx, req.Service()+"."+req.Endpoint())

	s.Type = trace.SpanTypeRequestOutbound
	err := c.Client.Call(newCtx, req, rsp, opts...)
	if err != nil {
		s.Metadata["error"] = err.Error()
	}

	// finish the trace
	debug.DefaultTracer.Finish(s)

	return err
}

// TraceCall is a call tracing wrapper
func TraceCall(c client.Client) client.Client {
	return &traceWrapper{
		Client: c,
	}
}

// TraceHandler wraps a server handler to perform tracing
func TraceHandler() server.HandlerWrapper {
	// return a handler wrapper
	return func(h server.HandlerFunc) server.HandlerFunc {
		// return a function that returns a function
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			// don't store traces for debug
			if strings.HasPrefix(req.Endpoint(), "Debug.") {
				return h(ctx, req, rsp)
			}

			// get the span
			newCtx, s := debug.DefaultTracer.Start(ctx, req.Service()+"."+req.Endpoint())
			s.Type = trace.SpanTypeRequestInbound

			err := h(newCtx, req, rsp)
			if err != nil {
				s.Metadata["error"] = err.Error()
			}

			// finish
			debug.DefaultTracer.Finish(s)

			return err
		}
	}
}

type cacheWrapper struct {
	Cache *cache.Cache
	client.Client
}

// Call executes the request. If the CacheExpiry option was set, the response will be cached using
// a hash of the metadata and request as the key.
func (c *cacheWrapper) Call(ctx context.Context, req client.Request, rsp interface{}, opts ...client.CallOption) error {
	// parse the options
	var options client.CallOptions
	for _, o := range opts {
		o(&options)
	}

	// if the client doesn't have a cacbe setup don't continue
	if c.Cache == nil {
		return c.Client.Call(ctx, req, rsp, opts...)
	}

	cacheOpts, ok := cache.GetOptions(options.Context)
	if !ok {
		return c.Client.Call(ctx, req, rsp, opts...)
	}

	// if the cache expiry is not set, execute the call without the cache
	if cacheOpts.Expiry == 0 || rsp == nil {
		return c.Client.Call(ctx, req, rsp, opts...)
	}

	// check to see if there is a response cached, if there is assign it
	if r, ok := c.Cache.Get(ctx, req); ok {
		val := reflect.ValueOf(rsp).Elem()
		val.Set(reflect.ValueOf(r).Elem())
		return nil
	}

	// don't cache the result if there was an error
	if err := c.Client.Call(ctx, req, rsp, opts...); err != nil {
		return err
	}

	// set the result in the cache
	c.Cache.Set(ctx, req, rsp, cacheOpts.Expiry)
	return nil
}

// CacheClient wraps requests with the cache wrapper
func CacheClient(c client.Client) client.Client {
	return &cacheWrapper{
		Cache:  cache.New(),
		Client: c,
	}
}

// MetricsHandler wraps a server handler to instrument calls
func MetricsHandler() server.HandlerWrapper {
	// return a handler wrapper
	return func(h server.HandlerFunc) server.HandlerFunc {
		// return a function that returns a function
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			// Don't instrument debug calls:
			if strings.HasPrefix(req.Endpoint(), "Debug.") {
				return h(ctx, req, rsp)
			}

			// Build some tags to describe the call:
			tags := metrics.Tags{
				"method": req.Method(),
			}

			// Start the clock:
			callTime := time.Now()

			// Run the handlerFunction:
			err := h(ctx, req, rsp)

			// Add a result tag:
			if err != nil {
				tags["result"] = "failure"
			} else {
				tags["result"] = "success"
			}

			// Instrument the result (if the DefaultClient has been configured):
			metrics.Timing("service.handler", time.Since(callTime), tags)

			return err
		}
	}
}
