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

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/lack-io/cli"

	"github.com/lack-io/vine/client/cli/namespace"
	"github.com/lack-io/vine/client/cli/util"
	"github.com/lack-io/vine/service/client"
	"github.com/lack-io/vine/service/context"
	"github.com/lack-io/vine/service/registry"
	goregistry "github.com/lack-io/vine/service/registry"
)

// lookupService queries the service for a service with the given alias. If
// no services are found for a given alias, the registry will return nil and
// the error will also be nil. An error is only returned if there was an issue
// listing from the registry.
func lookupService(ctx *cli.Context) (*goregistry.Service, string, error) {
	// use the first arg as the name, e.g. "vine helloworld foo"
	// would try to call the helloworld service
	name := ctx.Args().First()

	// if its a built in then we set domain to vine
	if util.IsBuiltInService(name) {
		srv, err := serviceWithName(name, goregistry.DefaultDomain)
		return srv, goregistry.DefaultDomain, err
	}

	env, err := util.GetEnv(ctx)
	if err != nil {
		return nil, "", err
	}
	// get the namespace to query the services from
	domain, err := namespace.Get(env.Name)
	if err != nil {
		return nil, "", err
	}

	// lookup from the registry in the current namespace
	if srv, err := serviceWithName(name, domain); err != nil {
		return nil, "", err
	} else if srv != nil {
		return srv, domain, nil
	}

	// if the request was made explicitly for the default
	// domain and we couldn't find it then just return nil
	if domain == goregistry.DefaultDomain {
		return nil, "", nil
	}

	// return a lookup in the default domain as a catch all
	srv, err := serviceWithName(name, goregistry.DefaultDomain)
	return srv, goregistry.DefaultDomain, err
}

// formatServiceUsage returns a string containing the service usage.
func formatServiceUsage(srv *goregistry.Service, c *cli.Context) string {
	alias := c.Args().First()
	subcommand := c.Args().Get(1)

	commands := make([]string, len(srv.Endpoints))
	endpoints := make([]*goregistry.Endpoint, len(srv.Endpoints))
	for i, e := range srv.Endpoints {
		// map "Helloworld.Call" to "helloworld.call"
		parts := strings.Split(e.Name, ".")
		for i, part := range parts {
			parts[i] = lowcaseInitial(part)
		}
		name := strings.Join(parts, ".")

		// remove the prefix if it is the service name, e.g. rather than
		// "vine run helloworld helloworld call", it would be
		// "vine run helloworld call".
		name = strings.TrimPrefix(name, alias+".")

		// instead of "vine run helloworld foo.bar", the command should
		// be "vine run helloworld foo bar".
		commands[i] = strings.Replace(name, ".", " ", 1)
		endpoints[i] = e
	}

	// sort the command names alphabetically
	sort.Strings(commands)

	result := ""
	if len(subcommand) > 0 && subcommand != "--help" {
		result += fmt.Sprintf("NAME:\n\tvine %v %v\n\n", alias, subcommand)
		result += fmt.Sprintf("USAGE:\n\tvine %v %v [flags]\n\n", alias, subcommand)
		result += fmt.Sprintf("FLAGS:\n")

		for i, command := range commands {
			if command == subcommand {
				result += renderFlags(endpoints[i])
			}
		}
	} else {
		result += fmt.Sprintf("NAME:\n\tvine %v\n\n", alias)
		result += fmt.Sprintf("VERSION:\n\t%v\n\n", srv.Version)
		result += fmt.Sprintf("USAGE:\n\tvine %v [command]\n\n", alias)
		result += fmt.Sprintf("COMMANDS:\n\t%v\n", strings.Join(commands, "\n\t"))

	}

	return result
}

func lowcaseInitial(str string) string {
	for i, v := range str {
		return string(unicode.ToLower(v)) + str[i+1:]
	}
	return ""
}

func renderFlags(endpoint *goregistry.Endpoint) string {
	ret := ""
	for _, value := range endpoint.Request.Values {
		ret += renderValue([]string{}, value) + "\n"
	}
	return ret
}

func renderValue(path []string, value *goregistry.Value) string {
	if len(value.Values) > 0 {
		renders := []string{}
		for _, v := range value.Values {
			renders = append(renders, renderValue(append(path, value.Name), v))
		}
		return strings.Join(renders, "\n")
	}
	return fmt.Sprintf("\t--%v %v", strings.Join(append(path, value.Name), "_"), value.Type)
}

// callService will call a service using the arguments and flags provided
// in the context. It will print the result or error to stdout. If there
// was an error performing the call, it will be returned.
func callService(srv *goregistry.Service, namespace string, ctx *cli.Context) error {
	// parse the flags and args
	args, flags, err := splitCmdArgs(ctx.Args().Slice())
	if err != nil {
		return err
	}

	// construct the endpoint
	endpoint, err := constructEndpoint(args)
	if err != nil {
		return err
	}

	// ensure the endpoint exists on the service
	var ep *goregistry.Endpoint
	for _, e := range srv.Endpoints {
		if e.Name == endpoint {
			ep = e
			break
		}
	}
	if ep == nil {
		return fmt.Errorf("Endpoint %v not found for service %v", endpoint, srv.Name)
	}

	// parse the flags
	body, err := flagsToRequest(flags, ep.Request)
	if err != nil {
		return err
	}

	// create a context for the call based on the cli context
	callCtx := ctx.Context

	// TODO: are we replacing a context that contains anything?
	if util.IsBuiltInService(srv.Name) {
		// replace with default for vine namespace in header
		callCtx = context.DefaultContext
	} else if len(namespace) > 0 {
		// set the namespace
		callCtx = context.SetNamespace(callCtx, namespace)
	}

	// TODO: parse out --header or --metadata

	// construct and execute the request using the json content type
	req := client.DefaultClient.NewRequest(srv.Name, endpoint, body, client.WithContentType("application/json"))
	var rsp json.RawMessage
	if err := client.DefaultClient.Call(callCtx, req, &rsp, client.WithAuthToken()); err != nil {
		return err
	}

	// format the response
	var out bytes.Buffer
	defer out.Reset()
	if err := json.Indent(&out, rsp, "", "\t"); err != nil {
		return err
	}
	out.Write([]byte("\n"))
	out.WriteTo(os.Stdout)

	return nil
}

// splitCmdArgs takes a cli context and parses out the args and flags, for
// example "vine helloworld --name=foo call apple" would result in "call",
// "apple" as args and {"name":"foo"} as the flags.
func splitCmdArgs(arguments []string) ([]string, map[string][]string, error) {
	args := []string{}
	flags := map[string][]string{}

	prev := ""
	for _, a := range arguments {
		if !strings.HasPrefix(a, "--") {
			if len(prev) == 0 {
				args = append(args, a)
				continue
			}
			_, exists := flags[prev]
			if !exists {
				flags[prev] = []string{}
			}

			flags[prev] = append(flags[prev], a)
			prev = ""
			continue
		}

		// comps would be "foo", "bar" for "--foo=bar"
		comps := strings.Split(strings.TrimPrefix(a, "--"), "=")
		_, exists := flags[comps[0]]
		if !exists {
			flags[comps[0]] = []string{}
		}
		switch len(comps) {
		case 1:
			prev = comps[0]
		case 2:
			flags[comps[0]] = append(flags[comps[0]], comps[1])
		default:
			return nil, nil, fmt.Errorf("Invalid flag: %v. Expected format: --foo=bar", a)
		}
	}

	return args, flags, nil
}

// constructEndpoint takes a slice of args and converts it into a valid endpoint
// such as Helloworld.Call or Foo.Bar, it will return an error if an invalid number
// of arguments were provided
func constructEndpoint(args []string) (string, error) {
	var epComps []string
	switch len(args) {
	case 1:
		epComps = append(args, "call")
	case 2:
		epComps = args
	case 3:
		epComps = args[1:3]
	default:
		return "", fmt.Errorf("Incorrect number of arguments")
	}

	// transform the endpoint components, e.g ["helloworld", "call"] to the
	// endpoint name: "Helloworld.Call".
	return fmt.Sprintf("%v.%v", strings.Title(epComps[0]), strings.Title(epComps[1])), nil
}

// shouldRenderHelp returns true if the help flag was passed
func shouldRenderHelp(ctx *cli.Context) bool {
	_, flags, _ := splitCmdArgs(ctx.Args().Slice())
	for key := range flags {
		if key == "help" {
			return true
		}
	}
	return false
}

// flagsToRequeest parses a set of flags, e.g {name:"Foo", "options_surname","Bar"} and
// converts it into a request body. If the key is not a valid object in the request, an
// error will be returned.
//
// This function constructs []interface{} slices
// as opposed to typed ([]string etc) slices for easier testing
func flagsToRequest(flags map[string][]string, req *goregistry.Value) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	coerceValue := func(valueType string, value []string) (interface{}, error) {
		switch valueType {
		case "bool":
			if len(value) == 0 || len(strings.TrimSpace(value[0])) == 0 {
				return true, nil
			}
			return strconv.ParseBool(value[0])
		case "int32":
			return strconv.Atoi(value[0])
		case "int64":
			return strconv.ParseInt(value[0], 0, 64)
		case "float64":
			return strconv.ParseFloat(value[0], 64)
		case "[]bool":
			// length is one if it's a `,` separated int slice
			if len(value) == 1 {
				value = strings.Split(value[0], ",")
			}
			ret := []interface{}{}
			for _, v := range value {
				i, err := strconv.ParseBool(v)
				if err != nil {
					return nil, err
				}
				ret = append(ret, i)
			}
			return ret, nil
		case "[]int32":
			// length is one if it's a `,` separated int slice
			if len(value) == 1 {
				value = strings.Split(value[0], ",")
			}
			ret := []interface{}{}
			for _, v := range value {
				i, err := strconv.Atoi(v)
				if err != nil {
					return nil, err
				}
				ret = append(ret, int32(i))
			}
			return ret, nil
		case "[]int64":
			// length is one if it's a `,` separated int slice
			if len(value) == 1 {
				value = strings.Split(value[0], ",")
			}
			ret := []interface{}{}
			for _, v := range value {
				i, err := strconv.ParseInt(v, 0, 64)
				if err != nil {
					return nil, err
				}
				ret = append(ret, i)
			}
			return ret, nil
		case "[]float64":
			// length is one if it's a `,` separated float slice
			if len(value) == 1 {
				value = strings.Split(value[0], ",")
			}
			ret := []interface{}{}
			for _, v := range value {
				i, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return nil, err
				}
				ret = append(ret, i)
			}
			return ret, nil
		case "[]string":
			// length is one it's a `,` separated string slice
			if len(value) == 1 {
				value = strings.Split(value[0], ",")
			}
			ret := []interface{}{}
			for _, v := range value {
				ret = append(ret, v)
			}
			return ret, nil
		case "string":
			return value[0], nil
		default:
			return value, nil
		}
		return nil, nil
	}
loop:
	for key, value := range flags {
		for _, attr := range req.Values {

			// matches at a top level
			if attr.Name == key {
				parsed, err := coerceValue(attr.Type, value)
				if err != nil {
					return nil, err
				}

				result[key] = parsed
				continue loop
			}

			// check for matches at the second level
			if !strings.HasPrefix(key, attr.Name+"_") {
				continue
			}
			for _, attr2 := range attr.Values {
				if attr.Name+"_"+attr2.Name != key {
					continue
				}

				if _, ok := result[attr.Name]; !ok {
					result[attr.Name] = map[string]interface{}{}
				} else if _, ok := result[attr.Name].(map[string]interface{}); !ok {
					return nil, fmt.Errorf("Error parsing request, duplicate key: %v", key)
				}
				parsed, err := coerceValue(attr2.Type, value)
				if err != nil {
					return nil, err
				}
				result[attr.Name].(map[string]interface{})[attr2.Name] = parsed
				continue loop
			}
		}
		return nil, fmt.Errorf("Unknown flag: %v", key)
	}

	return result, nil
}

// find a service in a domain matching the name
func serviceWithName(name, domain string) (*goregistry.Service, error) {
	srvs, err := registry.DefaultRegistry.GetService(name, goregistry.GetDomain(domain))
	if err == goregistry.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if len(srvs) == 0 {
		return nil, nil
	}
	return srvs[0], nil
}
