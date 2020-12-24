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

// Package host resolves using http host
package host

import (
	"net/http"

	"github.com/lack-io/vine/internal/api/resolver"
)

type Resolver struct {
	opts resolver.Options
}

func (r *Resolver) Resolve(req *http.Request) (*resolver.Endpoint, error) {
	return &resolver.Endpoint{
		Name:   req.Host,
		Host:   req.Host,
		Method: req.Method,
		Path:   req.URL.Path,
	}, nil
}

func (r *Resolver) String() string {
	return "host"
}

func NewResolver(opts ...resolver.Option) resolver.Resolver {
	return &Resolver{opts: resolver.NewOptions(opts...)}
}
