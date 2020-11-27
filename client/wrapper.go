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

package client

import (
	"context"

	"github.com/lack-io/vine/registry"
)

// CallFunc represents the individual call func
type CallFunc func(ctx context.Context, node *registry.Node, req Request, rsp interface{}, opts CallOptions) error

// CallWrapper is a low level wrapper for the CallFunc
type CallWrapper func(CallFunc) CallFunc

// Wrapper wraps a client and returns a client
type Wrapper func(Client) Client

// StreamWrapper wraps a Stream and returns the equivalent
type StreamWrapper func(Stream) Stream
