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

// Package new generates vine service templates
package new

import (
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/lack-io/cli"
	"github.com/xlab/treeprint"

	tmpl "github.com/lack-io/vine/cmd/vine/client/cli/new/template"
	"github.com/lack-io/vine/util/usage"
)

func protoComments(goDir, alias string) []string {
	return []string{
		"\ndownload protoc zip packages (protoc-$VERSION-$PLATFORM.zip) and install:\n",
		"visit https://github.com/protocolbuffers/protobuf/releases",
		"\ndownload protobuf for vine:\n",
		"go get -u github.com/gogo/protobuf",
		"go get github.com/gogo/googleapis",
		"go get -u github.com/lack-io/vine/cmd/protoc-gen-gogofaster",
		"go get github.com/lack-io/vine/cmd/protoc-gen-vine",
		"\ncompile the proto file " + alias + ".proto:\n",
		"cd " + goDir,
		"make proto\n",
	}
}

type config struct {
	// foo
	Alias string
	// vine new example -type
	Command string
	// go.vine
	Namespace string
	// api, service, web, function
	Type string
	// go.vine.service.foo
	FQDN string
	// github.com/vine/foo
	Dir string
	// $GOPATH/src/github.com/vine/foo
	GoDir string
	// $GOPATH
	GoPath string
	// UseGoPath
	UseGoPath bool
	// Files
	Files []file
	// Comments
	Comments []string
	// Plugins registry=etcd:broker=nats
	Plugins []string
}

type file struct {
	Path string
	Tmpl string
}

func write(c config, file, tmpl string) error {
	fn := template.FuncMap{
		"title": strings.Title,
	}

	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	t, err := template.New("f").Funcs(fn).Parse(tmpl)
	if err != nil {
		return err
	}

	return t.Execute(f, c)
}

func create(c config) error {
	// check if dir exists
	if _, err := os.Stat(c.GoDir); !os.IsNotExist(err) {
		return fmt.Errorf("%s already exists", c.GoDir)
	}

	// create usage report
	u := usage.New("new")
	// a single request/service
	u.Metrics.Count["requests"] = uint64(1)
	u.Metrics.Count["services"] = uint64(1)
	// send report
	go usage.Report(u)

	// just wait
	<-time.After(time.Millisecond * 250)

	fmt.Printf("Creating service %s in %s\n\n", c.FQDN, c.GoDir)

	t := treeprint.New()

	// write the files
	for _, file := range c.Files {
		f := filepath.Join(c.GoDir, file.Path)
		dir := filepath.Dir(f)

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}

		addFileToTree(t, file.Path)
		if err := write(c, f, file.Tmpl); err != nil {
			return err
		}
	}

	// print tree
	fmt.Println(t.String())

	for _, comment := range c.Comments {
		fmt.Println(comment)
	}

	// just wait
	<-time.After(time.Millisecond * 250)

	return nil
}

func addFileToTree(root treeprint.Tree, file string) {

	split := strings.Split(file, "/")
	curr := root
	for i := 0; i < len(split)-1; i++ {
		n := curr.FindByValue(split[i])
		if n != nil {
			curr = n
		} else {
			curr = curr.AddBranch(split[i])
		}
	}
	if curr.FindByValue(split[len(split)-1]) == nil {
		curr.AddNode(split[len(split)-1])
	}
}

func Run(ctx *cli.Context) {
	namespace := ctx.String("namespace")
	alias := ctx.String("alias")
	fqdn := ctx.String("fqdn")
	atype := ctx.String("type")
	dir := ctx.Args().First()
	useGoPath := ctx.Bool("gopath")
	useGoModule := os.Getenv("GO111MODULE")
	var plugins []string

	if len(dir) == 0 {
		fmt.Println("specify service name")
		return
	}

	if len(namespace) == 0 {
		fmt.Println("namespace not defined")
		return
	}

	if len(atype) == 0 {
		fmt.Println("type not defined")
		return
	}

	// set the command
	command := "vine new"
	if len(namespace) > 0 {
		command += " --namespace=" + namespace
	}
	if len(alias) > 0 {
		command += " --alias=" + alias
	}
	if len(fqdn) > 0 {
		command += " --fqdn=" + fqdn
	}
	if len(atype) > 0 {
		command += " --type=" + atype
	}
	if plugins := ctx.StringSlice("plugin"); len(plugins) > 0 {
		command += " --plugin=" + strings.Join(plugins, ":")
	}
	command += " " + dir

	// check if the path is absolute, we don't want this
	// we want to a relative path so we can install in GOPATH
	if path.IsAbs(dir) {
		fmt.Println("require relative path as service will be installed in GOPATH")
		return
	}

	var goPath string
	var goDir string

	// only set gopath if told to use it
	if useGoPath {
		goPath = build.Default.GOPATH

		// don't know GOPATH, runaway....
		if len(goPath) == 0 {
			fmt.Println("unknown GOPATH")
			return
		}

		// attempt to split path if not windows
		if runtime.GOOS == "windows" {
			goPath = strings.Split(goPath, ";")[0]
		} else {
			goPath = strings.Split(goPath, ":")[0]
		}
		goDir = filepath.Join(goPath, "src", path.Clean(dir))
	} else {
		goDir = path.Clean(dir)
	}

	if len(alias) == 0 {
		// set as last part
		alias = filepath.Base(dir)
		// strip hyphens
		parts := strings.Split(alias, "-")
		alias = parts[0]
	}

	if len(fqdn) == 0 {
		fqdn = strings.Join([]string{namespace, atype, alias}, ".")
	}

	for _, plugin := range ctx.StringSlice("plugin") {
		// registry=etcd:broker=nats
		for _, p := range strings.Split(plugin, ":") {
			// registry=etcd
			parts := strings.Split(p, "=")
			if len(parts) < 2 {
				continue
			}
			plugins = append(plugins, path.Join(parts...))
		}
	}

	c := config{
		Alias:     alias,
		Command:   command,
		Namespace: namespace,
		Type:      atype,
		FQDN:      fqdn,
		Dir:       dir,
		GoDir:     goDir,
		GoPath:    goPath,
		UseGoPath: useGoPath,
		Plugins:   plugins,
		Comments:  protoComments(goDir, alias),
	}

	switch atype {
	case "function":
		// create service config
		c.Files = []file{
			{"main.go", tmpl.MainFNC},
			{"generate.go", tmpl.GenerateFile},
			{"plugin.go", tmpl.Plugin},
			{"handler/" + alias + ".go", tmpl.HandlerFNC},
			{"subscriber/" + alias + ".go", tmpl.SubscriberFNC},
			{"proto/" + alias + "/" + alias + ".proto", tmpl.ProtoFNC},
			{"Dockerfile", tmpl.DockerFNC},
			{"Makefile", tmpl.Makefile},
			{"README.md", tmpl.ReadmeFNC},
			{".gitignore", tmpl.GitIgnore},
		}

	case "service":
		// create service config
		c.Files = []file{
			{"main.go", tmpl.MainSRV},
			{"generate.go", tmpl.GenerateFile},
			{"plugin.go", tmpl.Plugin},
			{"handler/" + alias + ".go", tmpl.HandlerSRV},
			{"subscriber/" + alias + ".go", tmpl.SubscriberSRV},
			{"proto/" + alias + "/" + alias + ".proto", tmpl.ProtoSRV},
			{"Dockerfile", tmpl.DockerSRV},
			{"Makefile", tmpl.Makefile},
			{"README.md", tmpl.Readme},
			{".gitignore", tmpl.GitIgnore},
		}
	case "api":
		// create api config
		c.Files = []file{
			{"main.go", tmpl.MainAPI},
			{"generate.go", tmpl.GenerateFile},
			{"plugin.go", tmpl.Plugin},
			{"client/" + alias + ".go", tmpl.WrapperAPI},
			{"handler/" + alias + ".go", tmpl.HandlerAPI},
			{"proto/" + alias + "/" + alias + ".proto", tmpl.ProtoAPI},
			{"Makefile", tmpl.Makefile},
			{"Dockerfile", tmpl.DockerSRV},
			{"README.md", tmpl.Readme},
			{".gitignore", tmpl.GitIgnore},
		}
	case "web":
		// create service config
		c.Files = []file{
			{"main.go", tmpl.MainWEB},
			{"plugin.go", tmpl.Plugin},
			{"handler/handler.go", tmpl.HandlerWEB},
			{"html/index.html", tmpl.HTMLWEB},
			{"Dockerfile", tmpl.DockerWEB},
			{"Makefile", tmpl.Makefile},
			{"README.md", tmpl.Readme},
			{".gitignore", tmpl.GitIgnore},
		}
		c.Comments = []string{}

	default:
		fmt.Println("Unknown type", atype)
		return
	}

	// set gomodule
	if useGoModule != "off" {
		c.Files = append(c.Files, file{"go.mod", tmpl.Module})
	}

	if err := create(c); err != nil {
		fmt.Println(err)
		return
	}

}

func Commands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "new",
			Usage: "Create a service template",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "namespace",
					Usage: "Namespace for the service e.g com.example",
					Value: "go.vine",
				},
				&cli.StringFlag{
					Name:  "type",
					Usage: "Type of service e.g api, function, service, web",
					Value: "service",
				},
				&cli.StringFlag{
					Name:  "fqdn",
					Usage: "FQDN of service e.g com.example.service.service (defaults to namespace.type.alias)",
				},
				&cli.StringFlag{
					Name:  "alias",
					Usage: "Alias is the short name used as part of combined name if specified",
				},
				&cli.StringSliceFlag{
					Name:  "plugin",
					Usage: "Specify plugins e.g --plugin=registry=etcd:broker=nats or use flag multiple times",
				},
				&cli.BoolFlag{
					Name:  "gopath",
					Usage: "Create the service in the gopath.",
					Value: true,
				},
			},
			Action: func(c *cli.Context) error {
				Run(c)
				return nil
			},
		},
	}
}
