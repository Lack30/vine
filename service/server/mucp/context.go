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

package mucp

import (
	"context"
	"sync"

	"github.com/lack-io/vine/service/server"
)

type serverKey struct{}

func wait(ctx context.Context) *sync.WaitGroup {
	if ctx == nil {
		return nil
	}
	wg, ok := ctx.Value("wait").(*sync.WaitGroup)
	if !ok {
		return nil
	}
	return wg
}

func FromContext(ctx context.Context) (server.Server, bool) {
	c, ok := ctx.Value(serverKey{}).(server.Server)
	return c, ok
}

func NewContext(ctx context.Context, s server.Server) context.Context {
	return context.WithValue(ctx, serverKey{}, s)
}