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

// Package proxy is a transport proxy built on the vine/server
package proxy

import (
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/router"
)

type Options struct {
	// Specific endpoint to always call
	Endpoint string
	// The default client to use
	Client client.Client
	// The default router to use
	Router router.Router
	// Extra links for different clients
	Links map[string]client.Client
}

type Option func(o *Options)

// WithEndpoint sets a proxy endpoint
func WithEndpoint(e string) Option {
	return func(o *Options) {
		o.Endpoint = e
	}
}

// WithClient sets the client
func WithClient(c client.Client) Option {
	return func(o *Options) {
		o.Client = c
	}
}

// WithRouter specifies the router to use
func WithRouter(r router.Router) Option {
	return func(o *Options) {
		o.Router = r
	}
}

// WithLink sets a link for outbound requests
func WithLink(name string, c client.Client) Option {
	return func(o *Options) {
		if o.Links == nil {
			o.Links = make(map[string]client.Client)
		}
		o.Links[name] = c
	}
}
