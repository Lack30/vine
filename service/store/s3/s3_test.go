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

package s3

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lack-io/vine/service/store"
)

func TestBlobStore(t *testing.T) {
	region := os.Getenv("S3_BLOB_STORE_REGION")
	if len(region) == 0 {
		t.Skipf("Missing required config S3_BLOB_STORE_REGION")
	}

	endpoint := os.Getenv("S3_BLOB_STORE_ENDPOINT")
	if len(endpoint) == 0 {
		t.Skipf("Missing required config S3_BLOB_STORE_ENDPOINT")
	}

	accessKey := os.Getenv("S3_BLOB_STORE_ACCESS_KEY")
	if len(accessKey) == 0 {
		t.Skipf("Missing required config S3_BLOB_STORE_ACCESS_KEY")
	}

	secretKey := os.Getenv("S3_BLOB_STORE_SECRET_KEY")
	if len(secretKey) == 0 {
		t.Skipf("Missing required config S3_BLOB_STORE_SECRET_KEY")
	}

	blob, err := NewBlobStore(
		Region(region),
		Endpoint(endpoint),
		Credentials(accessKey, secretKey),
	)
	assert.NotNilf(t, blob, "Blob should not be nil")
	assert.Nilf(t, err, "Error should be nil")
	if err != nil {
		return
	}

	t.Run("ReadMissingKey", func(t *testing.T) {
		res, err := blob.Read("")
		assert.Equal(t, store.ErrMissingKey, err, "Error should be missing key")
		assert.Nil(t, res, "Result should be nil")
	})

	t.Run("ReadNotFound", func(t *testing.T) {
		res, err := blob.Read("foo")
		assert.Equal(t, store.ErrNotFound, err, "Error should be not found")
		assert.Nil(t, res, "Result should be nil")
	})

	t.Run("WriteMissingKey", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte("HelloWorld"))
		err := blob.Write("", buf)
		assert.Equal(t, store.ErrMissingKey, err, "Error should be missing key")
	})

	t.Run("WriteValid", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte("world"))
		err := blob.Write("hello", buf)
		assert.Nilf(t, err, "Error should be nil")
	})

	t.Run("ReadValid", func(t *testing.T) {
		val, err := blob.Read("hello")
		assert.Nilf(t, err, "Error should be nil")
		assert.NotNilf(t, val, "Value should not be nil")

		if val != nil {
			bytes, err := ioutil.ReadAll(val)
			assert.Nilf(t, err, "Error should be nil")
			assert.Equal(t, "world", string(bytes), "Value should be world")
		}
	})

	t.Run("ReadIncorrectNamespace", func(t *testing.T) {
		val, err := blob.Read("hello", store.BlobNamespace("bar"))
		assert.Equal(t, store.ErrNotFound, err, "Error should be not found")
		assert.Nil(t, val, "Value should be nil")
	})

	t.Run("ReadCorrectNamespace", func(t *testing.T) {
		val, err := blob.Read("hello", store.BlobNamespace("vine"))
		assert.Nil(t, err, "Error should be nil")
		assert.NotNilf(t, val, "Value should not be nil")

		if val != nil {
			bytes, err := ioutil.ReadAll(val)
			assert.Nilf(t, err, "Error should be nil")
			assert.Equal(t, "world", string(bytes), "Value should be world")
		}
	})

	t.Run("DeleteIncorrectNamespace", func(t *testing.T) {
		err := blob.Delete("hello", store.BlobNamespace("bar"))
		assert.Nil(t, err, "Error should be nil")
	})

	t.Run("DeleteCorrectNamespaceIncorrectKey", func(t *testing.T) {
		err := blob.Delete("world", store.BlobNamespace("vine"))
		assert.Nil(t, err, "Error should be nil")
	})

	t.Run("DeleteCorrectNamespace", func(t *testing.T) {
		err := blob.Delete("hello", store.BlobNamespace("vine"))
		assert.Nil(t, err, "Error should be nil")
	})

	t.Run("ReadDeletedKey", func(t *testing.T) {
		res, err := blob.Read("hello", store.BlobNamespace("vine"))
		assert.Equal(t, store.ErrNotFound, err, "Error should be not found")
		assert.Nil(t, res, "Result should be nil")
	})
}
