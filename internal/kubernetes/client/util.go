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

package client

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/template"
)

// renderTemplateFile renders template for a given resource into writer w
func renderTemplate(resource string, w io.Writer, data interface{}) error {
	t := template.Must(template.New("kubernetes").Parse(templates[resource]))

	if err := t.Execute(w, data); err != nil {
		return err
	}

	return nil
}

// COPIED FROM
// https://github.com/kubernetes/kubernetes/blob/7a725418af4661067b56506faabc2d44c6d7703a/pkg/util/crypto/crypto.go

// CertPoolFromFile returns an x509.CertPool containing the certificates in the given PEM-encoded file.
// Returns an error if the file could not be read, a certificate could not be parsed, or if the file does not contain any certificates
func CertPoolFromFile(filename string) (*x509.CertPool, error) {
	certs, err := certificatesFromFile(filename)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	for _, cert := range certs {
		pool.AddCert(cert)
	}
	return pool, nil
}

// certificatesFromFile returns the x509.Certificates contained in the given PEM-encoded file.
// Returns an error if the file could not be read, a certificate could not be parsed, or if the file does not contain any certificates
func certificatesFromFile(file string) ([]*x509.Certificate, error) {
	if len(file) == 0 {
		return nil, errors.New("error reading certificates from an empty filename")
	}
	pemBlock, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	certs, err := CertsFromPEM(pemBlock)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %s", file, err)
	}
	return certs, nil
}

// CertsFromPEM returns the x509.Certificates contained in the given PEM-encoded byte array
// Returns an error if a certificate could not be parsed, or if the data does not contain any certificates
func CertsFromPEM(pemCerts []byte) ([]*x509.Certificate, error) {
	ok := false
	certs := []*x509.Certificate{}
	for len(pemCerts) > 0 {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}
		// Only use PEM "CERTIFICATE" blocks without extra headers
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs, err
		}

		certs = append(certs, cert)
		ok = true
	}

	if !ok {
		return certs, errors.New("could not read any certificates")
	}
	return certs, nil
}

// Format is used to format a string value into a k8s valid name
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
func Format(v string) string {
	// to lower case
	v = strings.ToLower(v)
	// / to dashes
	replaceChars := []string{"/", ".", "_"}
	for _, s := range replaceChars {
		v = strings.ReplaceAll(v, s, "-")
	}

	// limit to 253 chars
	if len(v) > 253 {
		v = v[:253]
	}
	// return new name
	return v
}
