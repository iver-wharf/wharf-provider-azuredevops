package httputils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
)

// NewClientWithCerts creates a fresh net/http.Client populated with some
// root CA certificates from file.
// Argument must point to an existing file with PEM formatted certificates.
//
// Based on https://forfuncsake.github.io/post/2017/08/trust-extra-ca-cert-in-go-app/
func NewClientWithCerts(localCertFile string) (*http.Client, error) {
	rootCAs, _ := x509.SystemCertPool()

	// Get the SystemCertPool, continue with an empty pool on error
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
		fmt.Println("using empty cert pool")
	} else {
		fmt.Println("using system cert pool")
	}

	// Read in the cert file
	certs, err := ioutil.ReadFile(localCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to append %q to RootCAs: %v", localCertFile, err)
	}

	fmt.Printf("loaded certs from %s\n", localCertFile)

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		fmt.Println("no certs appended, using system certs only")
	}

	// Trust the augmented cert pool in our client
	config := &tls.Config{
		RootCAs: rootCAs,
	}
	tr := &http.Transport{TLSClientConfig: config}
	client := &http.Client{Transport: tr}

	return client, nil
}
