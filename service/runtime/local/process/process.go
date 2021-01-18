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

// Package process executes a binary
package process

import (
	"io"

	"github.com/lack-io/vine/service/runtime/local/build"
)

// Process manages a running process
type Process interface {
	// Executes a process to completion
	Exec(*Executable) error
	// Creates a new process
	Fork(*Executable) (*PID, error)
	// Kills the process
	Kill(*PID) error
	// Waits for a process to exit
	Wait(*PID) error
}

type Executable struct {
	// Package containing executable
	Package *build.Package
	// The env variables
	Env []string
	// Args to pass
	Args []string
	// Initial working directory
	Dir string
}

// PID is the running process
type PID struct {
	// ID of the process
	ID string
	// Stdin
	Input io.Writer
	// Stdout
	Output io.Reader
	// Stderr
	Error io.Reader
}
