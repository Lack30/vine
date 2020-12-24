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
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/blang/semver"
	"github.com/lack-io/cli"
	"github.com/rhysd/go-github-selfupdate/selfupdate"

	"github.com/lack-io/vine/internal/config"
)

var (
	// SelfUpdate is set by gorelease LDFLAGS
	// We still prompt for update unless its disabled by env var
	// In future we may remove it entirely and always update
	SelfUpdate string
)

// confirmAndSelfUpdate looks for a new release of vine and upgrades in place
// we only execute this for select CLI commands rather than everything
func confirmAndSelfUpdate(ctx *cli.Context) (bool, error) {
	if SelfUpdate != "true" {
		return false, nil
	}

	// get the current version of the binary
	version := buildVersion()
	// we're going to update the binary
	update := true

	defer func() {
		// don't write new version unless told to
		if !update {
			return
		}

		// write the version at the end
		config.WriteVersion(version)
	}()

	// get the current version from .vine/version
	if ver, err := config.GetVersion(); err == nil {
		// check no more than once a day
		if !ver.Updated.IsZero() && time.Since(ver.Updated) < (time.Hour*24) {
			// don't update
			update = false
			return false, nil
		}
	}

	// look for an update
	latest, found, err := selfupdate.DetectLatest("vine/vine")
	if err != nil {
		return false, fmt.Errorf("Error occurred while detecting version: %s", err)
	}

	// check against the current version
	v, err := semver.ParseTolerant(buildVersion())
	if err != nil {
		return false, fmt.Errorf("Failed to parse build version: %v", err)
	}
	if !found || latest.Version.LTE(v) {
		// current version is the latest
		// write an update to state we checked
		return false, nil
	}

	// if its not enabled via the update prompt bail out
	if ctx.Bool("prompt_update") {
		fmt.Print("New version found. Do you want to update to ", latest.Version, "? (yes/no): ")
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil || (input != "yes\n" && input != "no\n") {
			return false, fmt.Errorf("Invalid response")
		}
		if input == "no\n" {
			return false, nil
		}
	} else {
		fmt.Println("New version detected. Updating now...")
	}

	exe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("Could not locate executable path")
	}
	if err := selfupdate.UpdateTo(latest.AssetURL, exe); err != nil {
		return false, fmt.Errorf("Error occurred while updating binary: %s", err)
	}

	// set the version, it'll be written at the very end
	version = latest.Version.String()

	fmt.Println("Successfully updated to version", latest.Version)
	return true, nil
}
