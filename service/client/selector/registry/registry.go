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

// Package registry uses the vine registry for selection
package registry

import (
	"github.com/lack-io/vine/service/client/selector"
)

// NewSelector returns a new registry selector
func NewSelector(opts ...selector.Option) selector.Selector {
	return newSelector(opts...)
}
