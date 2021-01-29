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

package bot

import (
	"github.com/lack-io/vine/service/agent/command"
)

type sortedCommands struct {
	commands []command.Command
}

func (s sortedCommands) Len() int {
	return len(s.commands)
}

func (s sortedCommands) Less(i, j int) bool {
	return s.commands[i].String() < s.commands[j].String()
}

func (s sortedCommands) Swap(i, j int) {
	s.commands[i], s.commands[j] = s.commands[j], s.commands[i]
}