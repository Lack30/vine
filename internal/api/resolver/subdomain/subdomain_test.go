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

package subdomain

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/lack-io/vine/internal/api/resolver/vpath"

	"github.com/stretchr/testify/assert"
)

func TestResolve(t *testing.T) {
	tt := []struct {
		Name   string
		Host   string
		Result string
	}{
		{
			Name:   "Top level domain",
			Host:   "vine.mu",
			Result: "vine",
		},
		{
			Name:   "Effective top level domain",
			Host:   "vine.com.au",
			Result: "vine",
		},
		{
			Name:   "Subdomain dev",
			Host:   "dev.vine.mu",
			Result: "dev",
		},
		{
			Name:   "Subdomain foo",
			Host:   "foo.vine.mu",
			Result: "foo",
		},
		{
			Name:   "Multi-level subdomain",
			Host:   "staging.myapp.m3o.app",
			Result: "myapp-staging",
		},
		{
			Name:   "Dev host",
			Host:   "127.0.0.1",
			Result: "vine",
		},
		{
			Name:   "Localhost",
			Host:   "localhost",
			Result: "vine",
		},
		{
			Name:   "IP host",
			Host:   "81.151.101.146",
			Result: "vine",
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			r := NewResolver(vpath.NewResolver())
			result, err := r.Resolve(&http.Request{URL: &url.URL{Host: tc.Host, Path: "foo/bar"}})
			assert.Nil(t, err, "Expecter err to be nil")
			if result != nil {
				assert.Equal(t, tc.Result, result.Domain, "Expected %v but got %v", tc.Result, result.Domain)
			}
		})
	}
}
