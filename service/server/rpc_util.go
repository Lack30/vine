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

package server

import "sync"

// waitgroup for global management connections
type waitGroup struct {
	// local waitgroup
	lg sync.WaitGroup
	// global waitgroup
	gg *sync.WaitGroup
}

func (w *waitGroup) Add(i int) {
	w.lg.Add(i)
	if w.gg != nil {
		w.gg.Add(i)
	}
}

func (w *waitGroup) Done() {
	w.lg.Done()
	if w.gg != nil {
		w.gg.Done()
	}
}

func (w *waitGroup) Wait() {
	// only wait on local group
	w.lg.Wait()
}
