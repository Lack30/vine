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

package namespace

import (
	"errors"
	"strings"

	"github.com/lack-io/vine/internal/config"
	"github.com/lack-io/vine/service/registry"
)

const seperator = ","

// List the namespaces for an environment
func List(env string) ([]string, error) {
	if len(env) == 0 {
		return nil, errors.New("Missing env value")
	}

	values, err := config.Get(config.Path("namespaces", env, "all"))
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return []string{registry.DefaultDomain}, nil
	}

	namespaces := strings.Split(values, seperator)
	return append([]string{registry.DefaultDomain}, namespaces...), nil
}

// Add a namespace to an environment
func Add(namespace, env string) error {
	if len(env) == 0 {
		return errors.New("Missing env value")
	}
	if len(namespace) == 0 {
		return errors.New("Missing namespace value")
	}

	existing, err := List(env)
	if err != nil {
		return err
	}
	for _, ns := range existing {
		if ns == namespace {
			// the namespace already exists
			return nil
		}
	}

	values, _ := config.Get(config.Path("namespaces", env, "all"))
	if len(values) > 0 {
		values = strings.Join([]string{values, namespace}, seperator)
	} else {
		values = namespace
	}

	return config.Set(config.Path("namespaces", env, "all"), values)
}

// Remove a namespace from an environment
func Remove(namespace, env string) error {
	if len(env) == 0 {
		return errors.New("Missing env value")
	}
	if len(namespace) == 0 {
		return errors.New("Missing namespace value")
	}
	if namespace == registry.DefaultDomain {
		return errors.New("Cannot remove the default namespace")
	}

	current, err := Get(env)
	if err != nil {
		return err
	}
	if current == namespace {
		err = Set(registry.DefaultDomain, env)
		if err != nil {
			return err
		}
	}

	existing, err := List(env)
	if err != nil {
		return err
	}

	var namespaces []string
	var found bool
	for _, ns := range existing {
		if ns == namespace {
			found = true
			continue
		}
		if ns == registry.DefaultDomain {
			continue
		}
		namespaces = append(namespaces, ns)
	}

	if !found {
		return errors.New("Namespace does not exists")
	}

	values := strings.Join(namespaces, seperator)
	return config.Set(config.Path("namespaces", env, "all"), values)
}

// Set the current namespace for an environment
func Set(namespace, env string) error {
	if len(env) == 0 {
		return errors.New("Missing env value")
	}
	if len(namespace) == 0 {
		return errors.New("Missing namespace value")
	}

	existing, err := List(env)
	if err != nil {
		return err
	}

	var found bool
	for _, ns := range existing {
		if ns != namespace {
			continue
		}
		found = true
		break
	}

	if !found {
		return errors.New("Namespace does not exists")
	}

	return config.Set(config.Path("namespaces", env, "current"), namespace)
}

// Get the current namespace for an environment
func Get(env string) (string, error) {
	if len(env) == 0 {
		return "", errors.New("Missing env value")
	}

	if ns, err := config.Get(config.Path("namespaces", env, "current")); err != nil {
		return "", err
	} else if len(ns) > 0 {
		return ns, nil
	}

	return registry.DefaultDomain, nil
}
