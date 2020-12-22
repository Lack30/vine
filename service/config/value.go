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

package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	simple "github.com/bitly/go-simplejson"
	json "github.com/json-iterator/go"
)

type JSONValues struct {
	values []byte
	sj     *simple.Json
}

type JSONValue struct {
	*simple.Json
}

func NewJSONValues(data []byte) *JSONValues {
	sj := simple.New()

	if err := sj.UnmarshalJSON(data); err != nil {
		sj.SetPath(nil, string(data))
	}
	return &JSONValues{data, sj}
}

func NewJSONValue(data []byte) *JSONValue {
	sj := simple.New()

	if err := sj.UnmarshalJSON(data); err != nil {
		sj.SetPath(nil, string(data))
	}
	return &JSONValue{sj}
}

func (j *JSONValues) Get(path string, options ...Option) Value {
	paths := strings.Split(path, ".")
	return &JSONValue{j.sj.GetPath(paths...)}
}

func (j *JSONValues) Delete(path string, options ...Option) {
	paths := strings.Split(path, ".")
	// delete the tree?
	if len(paths) == 0 {
		j.sj = simple.New()
		return
	}

	if len(paths) == 1 {
		j.sj.Del(paths[0])
		return
	}

	vals := j.sj.GetPath(paths[:len(paths)-1]...)
	vals.Del(paths[len(paths)-1])
	j.sj.SetPath(paths[:len(paths)-1], vals.Interface())
	return
}

func (j *JSONValues) Set(path string, val interface{}, options ...Option) {
	paths := strings.Split(path, ".")
	j.sj.SetPath(paths, val)
}

func (j *JSONValues) Bytes() []byte {
	b, _ := j.sj.MarshalJSON()
	return b
}

func (j *JSONValues) Map() map[string]interface{} {
	m, _ := j.sj.Map()
	return m
}

func (j *JSONValues) Scan(v interface{}) error {
	b, err := j.sj.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (j *JSONValues) String() string {
	return "json"
}

func (j *JSONValue) Bool(def bool) bool {
	b, err := j.Json.Bool()
	if err == nil {
		return b
	}

	str, ok := j.Interface().(string)
	if !ok {
		return def
	}

	b, err = strconv.ParseBool(str)
	if err != nil {
		return def
	}

	return b
}

func (j *JSONValue) Int(def int) int {
	i, err := j.Json.Int()
	if err == nil {
		return i
	}

	str, ok := j.Interface().(string)
	if !ok {
		return def
	}

	i, err = strconv.Atoi(str)
	if err != nil {
		return def
	}

	return i
}

func (j *JSONValue) String(def string) string {
	return j.Json.MustString(def)
}

func (j *JSONValue) Float64(def float64) float64 {
	f, err := j.Json.Float64()
	if err == nil {
		return f
	}

	str, ok := j.Interface().(string)
	if !ok {
		return def
	}

	f, err = strconv.ParseFloat(str, 64)
	if err != nil {
		return def
	}

	return f
}

func (j *JSONValue) Duration(def time.Duration) time.Duration {
	v, err := j.Json.String()
	if err != nil {
		return def
	}

	value, err := time.ParseDuration(v)
	if err != nil {
		return def
	}

	return value
}

func (j *JSONValue) StringSlice(def []string) []string {
	v, err := j.Json.String()
	if err == nil {
		sl := strings.Split(v, ",")
		if len(sl) > 1 {
			return sl
		}
	}
	return j.Json.MustStringArray(def)
}

func (j *JSONValue) Exists() bool {
	return false
}

func (j *JSONValue) StringMap(def map[string]string) map[string]string {
	m, err := j.Json.Map()
	if err != nil {
		return def
	}

	res := map[string]string{}

	for k, v := range m {
		res[k] = fmt.Sprintf("%v", v)
	}

	return res
}

func (j *JSONValue) Scan(v interface{}) error {
	b, err := j.Json.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (j *JSONValue) Bytes() []byte {
	b, err := j.Json.Bytes()
	if err != nil {
		// try return marshalled
		b, err = j.Json.MarshalJSON()
		if err != nil {
			return []byte{}
		}
		return b
	}
	return b
}
