package mongowire

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMongoWireAuthentication(t *testing.T) {
	address := os.Getenv("ARL_TEST_MONGO_AUTH_ADDR")
	if address == "" {
		t.Skip("set ARL_TEST_MONGO_AUTH_ADDR to run MongoDB wire integration tests")
	}
	user := os.Getenv("ARL_TEST_MONGO_USER")
	pass := os.Getenv("ARL_TEST_MONGO_PASSWORD")
	if user == "" || pass == "" {
		t.Fatal("ARL_TEST_MONGO_USER and ARL_TEST_MONGO_PASSWORD are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Ping(ctx, address); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := ProbeUnauthenticated(ctx, address); err == nil {
		t.Fatal("auth-enabled MongoDB accepted unauthenticated listDatabases")
	}
	if err := Authenticate(ctx, address, user, pass); err != nil {
		t.Fatalf("Authenticate valid credentials: %v", err)
	}
	if err := Authenticate(ctx, address, user, pass+"-wrong"); err == nil {
		t.Fatal("Authenticate accepted invalid credentials")
	}
}

func TestMongoWireNoAuthProbe(t *testing.T) {
	address := os.Getenv("ARL_TEST_MONGO_NOAUTH_ADDR")
	if address == "" {
		t.Skip("set ARL_TEST_MONGO_NOAUTH_ADDR to run MongoDB no-auth integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := Ping(ctx, address); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := ProbeUnauthenticated(ctx, address); err != nil {
		t.Fatalf("ProbeUnauthenticated: %v", err)
	}
}
