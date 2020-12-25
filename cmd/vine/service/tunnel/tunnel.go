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

package tunnel

import (
	"os"
	"strings"
	"time"

	"github.com/lack-io/cli"

	"github.com/lack-io/vine/service"
	"github.com/lack-io/vine/service/client"
	cmucp "github.com/lack-io/vine/service/client/mucp"
	log "github.com/lack-io/vine/service/logger"
	"github.com/lack-io/vine/service/proxy"
	"github.com/lack-io/vine/service/proxy/mucp"
	"github.com/lack-io/vine/service/registry/memory"
	"github.com/lack-io/vine/service/router"
	regRouter "github.com/lack-io/vine/service/router/registry"
	"github.com/lack-io/vine/service/server"
	smucp "github.com/lack-io/vine/service/server/mucp"
	"github.com/lack-io/vine/util/muxer"
	tun "github.com/lack-io/vine/util/network/tunnel"
	"github.com/lack-io/vine/util/network/tunnel/transport"
)

var (
	// Name of the tunnel service
	Name = "go.vine.tunnel"
	// Address is the tunnel address
	Address = ":8083"
	// Tunnel is the name of the tunnel
	Tunnel = "tun:0"
	// The tunnel token
	Token = "vine"
)

// Run runs the vine server
func Run(ctx *cli.Context, srvOpts ...service.Option) {
	// Init plugins
	for _, p := range Plugins() {
		p.Init(ctx)
	}

	if len(ctx.String("server-name")) > 0 {
		Name = ctx.String("server-name")
	}
	if len(ctx.String("address")) > 0 {
		Address = ctx.String("address")
	}
	if len(ctx.String("token")) > 0 {
		Token = ctx.String("token")
	}
	if len(ctx.String("id")) > 0 {
		Tunnel = ctx.String("id")
		// We need host:port for the Endpoint value in the proxy
		parts := strings.Split(Tunnel, ":")
		if len(parts) == 1 {
			Tunnel = Tunnel + ":0"
		}
	}
	var nodes []string
	if len(ctx.String("server")) > 0 {
		nodes = strings.Split(ctx.String("server"), ",")
	}

	// Initialise service
	srv := service.NewService(
		service.Name(Name),
		service.RegisterTTL(time.Duration(ctx.Int("register-ttl"))*time.Second),
		service.RegisterInterval(time.Duration(ctx.Int("register-interval"))*time.Second),
	)

	// local tunnel router
	r := regRouter.NewRouter(
		router.Id(srv.Server().Options().Id),
		router.Registry(srv.Client().Options().Registry),
	)

	// start the router
	if err := r.Start(); err != nil {
		log.Errorf("Tunnel error starting router: %s", err)
		os.Exit(1)
	}

	// create a tunnel
	t := tun.NewTunnel(
		tun.Address(Address),
		tun.Nodes(nodes...),
		tun.Token(Token),
	)

	// start the tunnel
	if err := t.Connect(); err != nil {
		log.Errorf("Tunnel error connecting: %v", err)
	}

	log.Infof("Tunnel [%s] listening on %s", Tunnel, Address)

	// create tunnel client with tunnel transport
	tunTransport := transport.NewTransport(
		transport.WithTunnel(t),
	)

	// local server client talks to tunnel
	localSrvClient := cmucp.NewClient(
		client.Transport(tunTransport),
	)

	// local proxy
	localProxy := mucp.NewProxy(
		proxy.WithClient(localSrvClient),
		proxy.WithEndpoint(Tunnel),
	)

	// create new muxer
	mux := muxer.New(Name, localProxy)

	// init server
	srv.Server().Init(
		server.WithRouter(mux),
	)

	// local transport client
	srv.Client().Init(
		client.Transport(srv.Options().Transport),
	)

	// local proxy
	tunProxy := mucp.NewProxy(
		proxy.WithRouter(r),
		proxy.WithClient(srv.Client()),
	)

	// create memory registry
	memRegistry := memory.NewRegistry()

	// local server
	tunSrv := smucp.NewServer(
		server.Address(Tunnel),
		server.Transport(tunTransport),
		server.WithRouter(tunProxy),
		server.Registry(memRegistry),
	)

	if err := tunSrv.Start(); err != nil {
		log.Errorf("Tunnel error starting tunnel server: %v", err)
		os.Exit(1)
	}

	if err := srv.Run(); err != nil {
		log.Errorf("Tunnel %s failed: %v", Name, err)
	}

	// stop the router
	if err := r.Stop(); err != nil {
		log.Errorf("Tunnel error stopping tunnel router: %v", err)
	}

	// stop the server
	if err := tunSrv.Stop(); err != nil {
		log.Errorf("Tunnel error stopping tunnel server: %v", err)
	}

	if err := t.Close(); err != nil {
		log.Errorf("Tunnel error stopping tunnel: %v", err)
	}
}

func Commands(options ...service.Option) []*cli.Command {
	command := &cli.Command{
		Name:  "tunnel",
		Usage: "Run the vine network tunnel",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "address",
				Usage:   "Set the vine tunnel address :8083",
				EnvVars: []string{"VINE_TUNNEL_ADDRESS"},
			},
			&cli.StringFlag{
				Name:    "id",
				Usage:   "Id of the tunnel used as the internal dial/listen address.",
				EnvVars: []string{"VINE_TUNNEL_ID"},
			},
			&cli.StringFlag{
				Name:    "server",
				Usage:   "Set the vine tunnel server address. This can be a comma separated list.",
				EnvVars: []string{"VINE_TUNNEL_SERVER"},
			},
			&cli.StringFlag{
				Name:    "token",
				Usage:   "Set the vine tunnel token for authentication",
				EnvVars: []string{"VINE_TUNNEL_TOKEN"},
			},
		},
		Action: func(ctx *cli.Context) error {
			Run(ctx, options...)
			return nil
		},
	}

	for _, p := range Plugins() {
		if cmds := p.Commands(); len(cmds) > 0 {
			command.Subcommands = append(command.Subcommands, cmds...)
		}

		if flags := p.Flags(); len(flags) > 0 {
			command.Flags = append(command.Flags, flags...)
		}
	}

	return []*cli.Command{command}
}