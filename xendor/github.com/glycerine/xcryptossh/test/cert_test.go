// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd

package test

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"

	"github.com/glycerine/sshego/xendor/github.com/glycerine/xcryptossh"
)

// Test both logging in with a cert, and also that the certificate presented by an OpenSSH host can be validated correctly
func TestCertLogin(t *testing.T) {
	ctx, cancelctx := context.WithCancel(context.Background())
	defer cancelctx()

	halt := ssh.NewHalter()
	defer halt.ReqStop.Close()

	s := newServer(t)
	defer s.Shutdown()

	// Use a key different from the default.
	clientKey := testSigners["dsa"]
	caAuthKey := testSigners["ecdsa"]
	cert := &ssh.Certificate{
		Key:             clientKey.PublicKey(),
		ValidPrincipals: []string{username()},
		CertType:        ssh.UserCert,
		ValidBefore:     ssh.CertTimeInfinity,
	}
	if err := cert.SignCert(rand.Reader, caAuthKey); err != nil {
		t.Fatalf("SetSignature: %v", err)
	}

	certSigner, err := ssh.NewCertSigner(cert, clientKey)
	if err != nil {
		t.Fatalf("NewCertSigner: %v", err)
	}

	conf := &ssh.ClientConfig{
		User: username(),
		HostKeyCallback: (&ssh.CertChecker{
			IsHostAuthority: func(pk ssh.PublicKey, addr string) bool {
				return bytes.Equal(pk.Marshal(), testPublicKeys["ca"].Marshal())
			},
		}).CheckHostKey,
		Config: ssh.Config{Halt: halt},
	}
	conf.Auth = append(conf.Auth, ssh.PublicKeys(certSigner))

	for _, test := range []struct {
		addr    string
		succeed bool
	}{
		{addr: "host.example.com:22", succeed: true},
		{addr: "host.example.com:10000", succeed: true}, // non-standard port must be OK
		{addr: "host.example.com", succeed: false},      // port must be specified
		{addr: "host.ex4mple.com:22", succeed: false},   // wrong host
	} {
		client, err := s.TryDialWithAddr(ctx, conf, test.addr)

		// Always close client if opened successfully
		if err == nil {
			client.Close()
		}

		// Now evaluate whether the test failed or passed
		if test.succeed {
			if err != nil {
				t.Fatalf("TryDialWithAddr: %v", err)
			}
		} else {
			if err == nil {
				t.Fatalf("TryDialWithAddr, unexpected success")
			}
		}
	}
}
