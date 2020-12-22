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

package plugin

import (
	"fmt"
	"sync"
)

const defaultModule = "vine"

type manager struct {
	sync.Mutex
	plugins    map[string][]Plugin
	registered map[string]map[string]bool
}

var (
	// global plugin manager
	defaultManager = newManager()
)

func newManager() *manager {
	return &manager{
		plugins:    make(map[string][]Plugin),
		registered: make(map[string]map[string]bool),
	}
}

func (m *manager) Plugins(opts ...PluginOption) []Plugin {
	options := PluginOptions{Module: defaultModule}
	for _, o := range opts {
		o(&options)
	}

	m.Lock()
	defer m.Unlock()

	if plugins, ok := m.plugins[options.Module]; ok {
		return plugins
	}
	return []Plugin{}
}

func (m *manager) Register(plugin Plugin, opts ...PluginOption) error {
	options := PluginOptions{Module: defaultModule}
	for _, o := range opts {
		o(&options)
	}

	m.Lock()
	defer m.Unlock()

	name := plugin.String()

	if reg, ok := m.registered[options.Module]; ok && reg[name] {
		return fmt.Errorf("Plugin with name %s already registered", name)
	}

	if _, ok := m.registered[options.Module]; !ok {
		m.registered[options.Module] = map[string]bool{name: true}
	} else {
		m.registered[options.Module][name] = true
	}

	if _, ok := m.plugins[options.Module]; ok {
		m.plugins[options.Module] = append(m.plugins[options.Module], plugin)
	} else {
		m.plugins[options.Module] = []Plugin{plugin}
	}

	return nil
}

func (m *manager) isRegistered(plugin Plugin, opts ...PluginOption) bool {
	options := PluginOptions{Module: defaultModule}
	for _, o := range opts {
		o(&options)
	}

	m.Lock()
	defer m.Unlock()

	if _, ok := m.registered[options.Module]; !ok {
		return false
	}

	return m.registered[options.Module][plugin.String()]
}
