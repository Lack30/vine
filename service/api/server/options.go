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

package server

import (
	"crypto/tls"
	"net/http"

	"github.com/lack-io/vine/service/api/resolver"
)

type Option func(o *Options)

type Options struct {
	EnableCORS   bool
	EnableTLS    bool
	TLSConfig    *tls.Config
	Resolver     resolver.Resolver
	Wrappers     []Wrapper
}

type Wrapper func(h http.Handler) http.Handler

func WrapHandler(w Wrapper) Option {
	return func(o *Options) {
		o.Wrappers = append(o.Wrappers, w)
	}
}

func EnableCORS(b bool) Option {
	return func(o *Options) {
		o.EnableCORS = b
	}
}

func EnableTLS(b bool) Option {
	return func(o *Options) {
		o.EnableTLS = b
	}
}

func TLSConfig(t *tls.Config) Option {
	return func(o *Options) {
		o.TLSConfig = t
	}
}

func Resolver(r resolver.Resolver) Option {
	return func(o *Options) {
		o.Resolver = r
	}
}