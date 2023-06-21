/*
Copyright 2022 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/appscodelabs/registry-redirect/pkg/redirect"

	flag "github.com/spf13/pflag"
	"golang.org/x/crypto/acme/autocert"
	"knative.dev/pkg/logging"
)

func main() {
	opts := redirect.NewOptions()
	opts.AddFlags(flag.CommandLine)
	flag.Parse()

	logger := logging.FromContext(context.Background())

	if !opts.EnableSSL {
		http.Handle("/", redirect.New(opts))

		logger.Infof("Listening on port %d", opts.Port)
		logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil))
		return
	}

	// ref:
	// - https://goenning.net/2017/11/08/free-and-automated-ssl-certificates-with-go/
	// - https://stackoverflow.com/a/40494806/244009
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(opts.CertDir),
		HostPolicy: autocert.HostWhitelist(opts.Hosts...),
		Email:      opts.CertEmail,
	}
	server := &http.Server{
		Addr:         ":https",
		Handler:      redirect.New(opts),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	go func() {
		// does automatic http to https redirects
		err := http.ListenAndServe(":http", certManager.HTTPHandler(nil))
		if err != nil {
			panic(err)
		}
	}()
	logger.Fatal(server.ListenAndServeTLS("", "")) // Key and cert are coming from Let's Encrypt
}
