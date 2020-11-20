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

package log

// Logger is a generic logging interface
type Logger interface {
	Debug(args ...interface{})

	Debugf(format string, v ...interface{})

	Info(args ...interface{})

	Infof(format string, v ...interface{})

	Warn(args ...interface{})

	Warnf(format string, v ...interface{})

	Error(args ...interface{})

	Errorf(format string, v ...interface{})

	Fatal(args ...interface{})

	Fatalf(format string, v ...interface{})
}
