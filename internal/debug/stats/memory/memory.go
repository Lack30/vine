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

package stats

import (
	"runtime"
	"sync"
	"time"

	"github.com/lack-io/vine/internal/debug/stats"
	"github.com/lack-io/vine/internal/ring"
)

type memoryStats struct {
	// used to store past stats
	buffer *ring.Buffer

	sync.RWMutex
	started  int64
	requests uint64
	errors   uint64
}

func (s *memoryStats) snapshot() *stats.Stat {
	s.RLock()
	defer s.RUnlock()

	var mstat runtime.MemStats
	runtime.ReadMemStats(&mstat)

	now := time.Now().Unix()

	return &stats.Stat{
		Timestamp: now,
		Started:   s.started,
		Uptime:    now - s.started,
		Memory:    mstat.Alloc,
		GC:        mstat.PauseTotalNs,
		Threads:   uint64(runtime.NumGoroutine()),
		Requests:  s.requests,
		Errors:    s.errors,
	}
}

func (s *memoryStats) Read() ([]*stats.Stat, error) {
	buf := s.buffer.Get(s.buffer.Size())
	var buffer []*stats.Stat

	// get a value from the buffer if it exists
	for _, b := range buf {
		stat, ok := b.Value.(*stats.Stat)
		if !ok {
			continue
		}
		buffer = append(buffer, stat)
	}

	// get a snapshot
	buffer = append(buffer, s.snapshot())

	return buffer, nil
}

func (s *memoryStats) Write(stat *stats.Stat) error {
	s.buffer.Put(stat)
	return nil
}

func (s *memoryStats) Record(err error) error {
	s.Lock()
	defer s.Unlock()

	// increment the total request count
	s.requests++

	// increment the error count
	if err != nil {
		s.errors++
	}

	return nil
}

// NewStats returns a new in memory stats buffer
// TODO add options
func NewStats() stats.Stats {
	return &memoryStats{
		started: time.Now().Unix(),
		buffer:  ring.New(1),
	}
}
