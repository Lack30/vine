// Copyright 2020 lack
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

package memory

import (
	"errors"

	regpb "github.com/lack-io/vine/proto/registry"
	"github.com/lack-io/vine/service/registry"
)

type memoryWatcher struct {
	exit chan bool
	opts registry.WatchOptions
}

func (m *memoryWatcher) Next() (*regpb.Result, error) {
	// not implement so we just block util exit
	<-m.exit
	return nil, errors.New("watcher stopped")
}

func (m *memoryWatcher) Stop() {
	select {
	case <-m.exit:
		return
	default:
		close(m.exit)
	}
}
