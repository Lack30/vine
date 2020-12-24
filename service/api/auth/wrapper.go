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

package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lack-io/vine/internal/api/resolver"
	"github.com/lack-io/vine/internal/api/resolver/subdomain"
	"github.com/lack-io/vine/internal/api/server"
	inauth "github.com/lack-io/vine/internal/auth"
	"github.com/lack-io/vine/internal/ctx"
	"github.com/lack-io/vine/internal/namespace"
	"github.com/lack-io/vine/service/auth"
	"github.com/lack-io/vine/service/logger"
)

// Wrapper wraps a handler and authenticates requests
func Wrapper(r resolver.Resolver, prefix string) server.Wrapper {
	return func(h http.Handler) http.Handler {
		return authWrapper{
			handler:       h,
			resolver:      r,
			servicePrefix: prefix,
		}
	}
}

type authWrapper struct {
	handler       http.Handler
	resolver      resolver.Resolver
	servicePrefix string
}

func (a authWrapper) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Determine the name of the service being requested
	endpoint, err := a.resolver.Resolve(req)
	if err == resolver.ErrInvalidPath || err == resolver.ErrNotFound {
		// a file not served by the resolver has been requested (e.g. favicon.ico)
		endpoint = &resolver.Endpoint{Path: req.URL.Path}
	} else if err != nil {
		logger.Error(err)
		http.Error(w, err.Error(), 500)
		return
	} else {
		// set the endpoint in the context so it can be used to resolve
		// the request later
		ctx := context.WithValue(req.Context(), resolver.Endpoint{}, endpoint)
		*req = *req.Clone(ctx)
	}

	// If an error occured looking up the route, the domain isn't returned. TODO: Find a better way
	// of resolving network for non-standard requests, e.g. "/rpc".
	if r, ok := a.resolver.(*subdomain.Resolver); ok && len(endpoint.Domain) == 0 {
		endpoint.Domain = r.Domain(req)
	}

	// Set the metadata so we can access it in vine api / web
	req = req.WithContext(ctx.FromRequest(req))

	// Extract the token from the request
	var token string
	if header := req.Header.Get("Authorization"); len(header) > 0 {
		// Extract the auth token from the request
		if strings.HasPrefix(header, inauth.BearerScheme) {
			token = header[len(inauth.BearerScheme):]
		}
	} else {
		// Get the token out the cookies if not provided in headers
		if c, err := req.Cookie("vine-token"); err == nil && c != nil {
			token = strings.TrimPrefix(c.Value, inauth.TokenCookieName+"=")
			req.Header.Set("Authorization", inauth.BearerScheme+token)
		}
	}

	// Get the account using the token, some are unauthenticated, so the lack of an
	// account doesn't necesserially mean a forbidden request
	acc, _ := auth.Inspect(token)

	// Determine the namespace and set it in the header. If the user passed auth creds
	// on the request, use the namespace that issued the account, otherwise check for
	// the domain of the resolved endpoint.
	ns := req.Header.Get(namespace.NamespaceKey)
	if len(ns) == 0 && acc != nil {
		ns = acc.Issuer
		req.Header.Set(namespace.NamespaceKey, ns)
	} else if len(ns) == 0 {
		ns = endpoint.Domain
		req.Header.Set(namespace.NamespaceKey, ns)
	}

	// Ensure accounts only issued by the namesace are valid
	if acc != nil && acc.Issuer != ns {
		acc = nil
	}

	// construct the resource name, e.g. home => foo.api.home
	resName := endpoint.Name
	if len(a.servicePrefix) > 0 {
		resName = a.servicePrefix + "." + resName
	}

	// determine the resource path. there is an inconsistency in how resolvers
	// use method, some use it as Users.ReadUser (the rpc method), and others
	// use it as the HTTP method, e.g GET. TODO: Refactor this to make it consistent.
	resEndpoint := endpoint.Path
	if len(endpoint.Path) == 0 {
		resEndpoint = endpoint.Method
	}

	// Options to use when verifying the request
	verifyOpts := []auth.VerifyOption{
		auth.VerifyContext(req.Context()),
		auth.VerifyNamespace(ns),
	}

	// Perform the verification check to see if the account has access to
	// the resource they're requesting
	res := &auth.Resource{Type: "service", Name: resName, Endpoint: resEndpoint}
	if err := auth.Verify(acc, res, verifyOpts...); err == nil {
		// The account has the necessary permissions to access the resource
		a.handler.ServeHTTP(w, req)
		return
	} else if err != auth.ErrForbidden {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// The account is set, but they don't have enough permissions, hence
	// we return a forbidden error.
	if acc != nil {
		http.Error(w, "Forbidden request", http.StatusForbidden)
		return
	}

	// If there is no auth login url set, 401
	loginURL := auth.DefaultAuth.Options().LoginURL
	if loginURL == "" {
		http.Error(w, "unauthorized request", http.StatusUnauthorized)
		return
	}

	// Redirect to the login path
	params := url.Values{"redirect_to": {req.URL.String()}}
	loginWithRedirect := fmt.Sprintf("%v?%v", loginURL, params.Encode())
	http.Redirect(w, req, loginWithRedirect, http.StatusTemporaryRedirect)
}
