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

package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"

	pb "github.com/lack-io/vine/proto/runtime"
	"github.com/lack-io/vine/service/auth"
	"github.com/lack-io/vine/service/errors"
	"github.com/lack-io/vine/service/store"
	gostore "github.com/lack-io/vine/service/store"
)

const (
	sourcePrefix        = "source://"
	blobNamespacePrefix = "vine/runtime"
)

// Source implements the proto source service interface
type Source struct{}

// Upload source to the server
func (s *Source) Upload(ctx context.Context, stream pb.Source_UploadStream) error {
	// authorize the request
	acc, ok := auth.AccountFromContext(ctx)
	if !ok {
		return errors.Unauthorized("runtime.Source.Upload", "An account is required to upload source")
	}
	namespace := acc.Issuer

	// recieve the source from the client
	buf := bytes.NewBuffer(nil)
	var srv *pb.Service
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.InternalServerError("runtime.Source.Upload", err.Error())
		}

		// get the service from the request, this should be sent on the first message
		if req.Service != nil {
			srv = req.Service
		}

		// write the bytes to the buffer
		if _, err := buf.Write(req.Data); err != nil {
			return err
		}
	}

	// ensure the blob and a service was sent over the stream
	if buf == nil {
		return errors.BadRequest("runtime.Source.Upload", "No blob was sent")
	}
	if srv == nil {
		return errors.BadRequest("runtime.Source.Upload", "No service was sent")
	}

	// write the source to the store
	key := fmt.Sprintf("source://%v:%v", srv.Name, srv.Version)
	opt := gostore.BlobNamespace(namespace)
	if err := store.DefaultBlobStore.Write(key, buf, opt); err != nil {
		return fmt.Errorf("Error writing source to blob store: %v", err)
	}

	// close the stream
	return stream.SendAndClose(&pb.UploadResponse{Id: key})
}
