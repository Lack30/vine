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

package proto

import (
	"io"
	"io/ioutil"

	"github.com/gogo/protobuf/proto"

	"github.com/lack-io/vine/service/codec"
)

type Codec struct {
	Conn io.ReadWriteCloser
}

func (c *Codec) ReadHeader(m *codec.Message, t codec.MessageType) error {
	return nil
}

func (c *Codec) ReadBody(b interface{}) error {
	if b == nil {
		return nil
	}
	buf, err := ioutil.ReadAll(c.Conn)
	if err != nil {
		return err
	}
	m, ok := b.(proto.Message)
	if !ok {
		return codec.ErrInvalidMessage
	}
	return proto.Unmarshal(buf, m)
}

func (c *Codec) Write(m *codec.Message, b interface{}) error {
	p, ok := b.(proto.Message)
	if !ok {
		return codec.ErrInvalidMessage
	}
	buf, err := proto.Marshal(p)
	if err != nil {
		return err
	}
	_, err = c.Conn.Write(buf)
	return err
}

func (c *Codec) Close() error {
	return c.Conn.Close()
}

func (c *Codec) String() string {
	return "proto"
}

func NewCodec(c io.ReadWriteCloser) codec.Codec {
	return &Codec{
		Conn: c,
	}
}
