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

package golang

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lack-io/vine/service/build"
)

var (
	testMainGo   = "package main; import \"fmt\"; func main() { fmt.Println(\"HelloWorld\") }"
	testSecondGo = "package main; import \"fmt\"; func init() { fmt.Println(\"Init\") }"
)

func TestGolangBuilder(t *testing.T) {
	t.Run("NoArchive", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte(testMainGo))
		err := testBuilder(t, buf)
		assert.Nil(t, err, "No error should be returned")
	})

	t.Run("InvalidArchive", func(t *testing.T) {
		buf := bytes.NewBuffer([]byte(testMainGo))
		err := testBuilder(t, buf, build.Archive("foo"))
		assert.Error(t, err, "An error should be returned")
	})

	t.Run("TarArchive", func(t *testing.T) {
		// Create a tar writer
		tf := bytes.NewBuffer(nil)
		tw := tar.NewWriter(tf)

		// Add some files to the archive.
		var files = []struct {
			Name, Body string
		}{
			{"main.go", testMainGo},
			{"second.go", testSecondGo},
		}
		for _, file := range files {
			hdr := &tar.Header{
				Name: file.Name,
				Mode: 0600,
				Size: int64(len(file.Body)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}

		err := testBuilder(t, tf, build.Archive("tar"))
		assert.Nil(t, err, "No error should be returned")
	})

	t.Run("ZipArchive", func(t *testing.T) {
		// Create a buffer to write our archive to.
		buf := new(bytes.Buffer)

		// Create a new zip archive.
		w := zip.NewWriter(buf)
		defer w.Close()

		// Add some files to the archive.
		var files = []struct {
			Name, Body string
		}{
			{"main.go", testMainGo},
			{"second.go", testSecondGo},
		}
		for _, file := range files {
			f, err := w.Create(file.Name)
			if err != nil {
				t.Fatal(err)
			}
			_, err = f.Write([]byte(file.Body))
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}

		err := testBuilder(t, buf, build.Archive("zip"))
		assert.Nil(t, err, "No error should be returned")
	})
}

func testBuilder(t *testing.T, buf io.Reader, opts ...build.Option) error {
	// setup the build
	build, err := NewBuilder()
	if err != nil {
		return fmt.Errorf("Error creating the build: %v", err)
	}

	// build the source
	res, err := build.Build(buf, opts...)
	if err != nil {
		return fmt.Errorf("Error building source: %v", err)
	}

	// write the binary to a tmp file and make it executable
	file, err := ioutil.TempFile(os.TempDir(), "res")
	if err != nil {
		return fmt.Errorf("Error creating tmp output file: %v", err)
	}
	if _, err := io.Copy(file, res); err != nil {
		return fmt.Errorf("Error copying binary to tmp file: %v", err)
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(file.Name(), 0111); err != nil {
		return err
	}
	defer os.Remove(file.Name())

	// execute the binary
	cmd := exec.Command(file.Name())
	outp, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Error executing binary: %v", err)
	}
	if !strings.Contains(string(outp), "HelloWorld") {
		return fmt.Errorf("Output does not contain HelloWorld")
	}
	// when an archive is used we also check for the second file to be loaded
	if len(opts) > 0 && !strings.Contains(string(outp), "Init") {
		return fmt.Errorf("Output does not contain Init")
	}

	return nil
}
