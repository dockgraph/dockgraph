package main

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"

	"github.com/dockgraph/dockgraph/agentauth"
)

func testAgentKey(t *testing.T) (ed25519.PrivateKey, []ed25519.PublicKey) {
	t.Helper()
	text, err := agentauth.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	key, _ := agentauth.ParsePrivateKey(text)
	pub, _ := agentauth.ParsePublicKeys(agentauth.PublicKeyText(key))
	return key, pub
}

// --agent-keygen's output piped into --agent-pubkey gives the matching
// public key.
func TestKeyCommands(t *testing.T) {
	var private bytes.Buffer
	if err := runKeyCommand(cmdAgentKeygen, nil, &private); err != nil {
		t.Fatal(err)
	}
	key, err := agentauth.ParsePrivateKey(private.String())
	if err != nil {
		t.Fatalf("keygen output: %v", err)
	}

	var public bytes.Buffer
	if err := runKeyCommand(cmdAgentPubkey, &private, &public); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(public.String()), agentauth.PublicKeyText(key); got != want {
		t.Errorf("pubkey = %q, want %q", got, want)
	}

	if err := runKeyCommand(cmdAgentPubkey, strings.NewReader("garbage"), &public); err == nil {
		t.Error("--agent-pubkey accepted garbage")
	}
	if err := runKeyCommand("--other", nil, &public); err == nil {
		t.Error("unknown command accepted")
	}
}

func TestServerAgentAuth(t *testing.T) {
	key, pub := testAgentKey(t)
	for _, tc := range []struct {
		name   string
		cfg    Config
		want   string // type of the authorizer, "" for none
		notice string
	}{
		{"key", Config{AgentKey: key}, "*agentauth.Signer", "signed"},
		{"key and token", Config{AgentKey: key, AgentToken: "tok"}, "agentauth.Chain", "still carry"},
		{"token only", Config{AgentToken: "tok"}, "agentauth.Token", "WARN"},
		{"public key only", Config{AgentPublicKeys: pub}, "", "server needs the private key"},
		{"nothing", Config{}, "", "agents disabled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, notice := serverAgentAuth(tc.cfg)
			if got := typeName(a); got != tc.want {
				t.Errorf("authorizer %s, want %s", got, tc.want)
			}
			if !strings.Contains(notice, tc.notice) {
				t.Errorf("notice %q lacks %q", notice, tc.notice)
			}
		})
	}
}

func typeName(a agentauth.Authorizer) string {
	switch a.(type) {
	case nil:
		return ""
	case *agentauth.Signer:
		return "*agentauth.Signer"
	case agentauth.Chain:
		return "agentauth.Chain"
	case agentauth.Token:
		return "agentauth.Token"
	}
	return "unknown"
}

func TestAgentAuthNotice(t *testing.T) {
	key, pub := testAgentKey(t)
	if _, err := agentAuthNotice(Config{AgentKey: key, AgentPublicKeys: pub}); !errors.Is(err, errAgentHasPrivateKey) {
		t.Errorf("agent given the private key: err = %v", err)
	}
	if n, _ := agentAuthNotice(Config{AgentPublicKeys: pub, AgentToken: "tok"}); !strings.Contains(n, "DG_AGENT_TOKEN is ignored") {
		t.Errorf("keys and token: notice %q", n)
	}
	if n, _ := agentAuthNotice(Config{AgentPublicKeys: pub}); !strings.Contains(n, "signed by the server") {
		t.Errorf("keys: notice %q", n)
	}
	if n, _ := agentAuthNotice(Config{AgentToken: "tok"}); !strings.HasPrefix(n, "WARN") {
		t.Errorf("token only: notice %q", n)
	}
}
