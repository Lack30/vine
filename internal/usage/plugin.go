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

package usage

import (
	"math/rand"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/lack-io/cli"

	"github.com/lack-io/vine/internal/backoff"
	"github.com/lack-io/vine/plugin"
	"github.com/lack-io/vine/service/registry"
)

func init() {
	plugin.Register(Plugin())
}

func Plugin() plugin.Plugin {
	var requests uint64

	// create rand
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)

	return plugin.NewPlugin(
		plugin.WithName("usage"),
		plugin.WithInit(func(c *cli.Context) error {
			// only do if enabled
			if !c.Bool("report_usage") {
				os.Setenv("VINE_REPORT_USAGE", "false")
				return nil
			}

			var service string

			// set service name
			if c.Args().Len() > 0 && len(c.Args().Get(0)) > 0 {
				service = c.Args().Get(0)
			}

			// service subcommand
			if service == "service" {
				// set as the sub command
				if v := c.Args().Get(1); len(v) > 0 {
					service = v
				}
			}

			// kick off the tracker
			go func() {
				// new report
				u := New(service)

				// initial publish in 30-60 seconds
				d := 30 + r.Intn(30)
				time.Sleep(time.Second * time.Duration(d))

				for {
					// get service list
					s, _ := registry.ListServices()
					// get requests
					reqs := atomic.LoadUint64(&requests)
					srvs := uint64(len(s))

					// reset requests
					atomic.StoreUint64(&requests, 0)

					// set metrics
					u.Metrics.Count["instances"] = uint64(1)
					u.Metrics.Count["requests"] = reqs
					u.Metrics.Count["services"] = srvs

					// attempt to send report 3 times
					for i := 1; i <= 3; i++ {
						if err := Report(u); err != nil {
							time.Sleep(backoff.Do(i * 2))
							continue
						}
						break
					}

					// now sleep 24 hours
					time.Sleep(time.Hour * 24)
				}
			}()

			return nil
		}),
		plugin.WithHandler(func(h http.Handler) http.Handler {
			// only enable if set
			if v := os.Getenv("VINE_REPORT_USAGE"); v == "false" {
				return h
			}

			// return usage recorder
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// count requests
				atomic.AddUint64(&requests, 1)
				// serve the request
				h.ServeHTTP(w, r)
			})
		}),
	)
}
