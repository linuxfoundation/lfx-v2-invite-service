// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
)

// startNkeyNATSServer launches an embedded NATS server that only accepts the
// given nkey user, so an anonymous or wrong-key connection is refused.
func startNkeyNATSServer(t *testing.T, pub string) string {
	t.Helper()
	opts := &natsserver.Options{
		Host:   "127.0.0.1",
		Port:   -1,
		NoLog:  true,
		NoSigs: true,
		Nkeys:  []*natsserver.NkeyUser{{Nkey: pub}},
	}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatalf("create NATS server: %v", err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(4 * time.Second) {
		t.Fatal("NATS server not ready")
	}
	t.Cleanup(ns.Shutdown)
	return ns.ClientURL()
}

func newUserSeed(t *testing.T) (seed, pub string) {
	t.Helper()
	kp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("create user nkey: %v", err)
	}
	s, err := kp.Seed()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	p, err := kp.PublicKey()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	return string(s), p
}

func TestNew_NkeySeedAuthenticates(t *testing.T) {
	seed, pub := newUserSeed(t)
	url := startNkeyNATSServer(t, pub)

	c, err := New(context.Background(), url, seed)
	if err != nil {
		t.Fatalf("New with nkey seed: %v", err)
	}
	defer c.Close()
	if err := c.conn.FlushTimeout(2 * time.Second); err != nil {
		t.Fatalf("connection not usable: %v", err)
	}
}

func TestNew_EmptySeedConnectsAnonymously(t *testing.T) {
	c, err := New(context.Background(), startTestNATSServer(t), "")
	if err != nil {
		t.Fatalf("New without seed: %v", err)
	}
	defer c.Close()
	if err := c.IsReady(); err != nil {
		t.Fatalf("anonymous connection not ready: %v", err)
	}
}

func TestNkeyOption_RejectsBadSeeds(t *testing.T) {
	accountKP, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("create account nkey: %v", err)
	}
	accountSeed, err := accountKP.Seed()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, userPub := newUserSeed(t)

	for name, seed := range map[string]string{
		"garbage":      "not-a-seed",
		"public key":   userPub,
		"account seed": string(accountSeed),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := nkeyOption(seed); err == nil {
				t.Fatalf("nkeyOption(%s) succeeded, want error", name)
			}
		})
	}
}
