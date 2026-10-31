package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/dockgraph/dockgraph/agentauth"
)

// Commands managing the key pair the server signs agent requests with.
const (
	cmdAgentKeygen = "--agent-keygen"
	cmdAgentPubkey = "--agent-pubkey"
)

// maxKeyInput bounds what --agent-pubkey reads from stdin.
const maxKeyInput = 4096

// runKeyCommand runs --agent-keygen, which prints a new private key for the
// server, or --agent-pubkey, which reads that private key from in and
// prints the public key for the agents.
func runKeyCommand(cmd string, in io.Reader, out io.Writer) error {
	switch cmd {
	case cmdAgentKeygen:
		key, err := agentauth.GenerateKey()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, key)
		return err
	case cmdAgentPubkey:
		data, err := io.ReadAll(io.LimitReader(in, maxKeyInput))
		if err != nil {
			return fmt.Errorf("reading the private key from stdin: %w", err)
		}
		key, err := agentauth.ParsePrivateKey(string(data))
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, agentauth.PublicKeyText(key))
		return err
	}
	return fmt.Errorf("unknown command %q", cmd)
}

// serverAgentAuth returns how the server authenticates to its agents and
// the line to log about it. With the key it signs every request, and also
// sends the token when one is set, for agents not given the public key yet.
// A nil Authorizer means no credentials: agents are not used.
func serverAgentAuth(cfg Config) (agentauth.Authorizer, string) {
	switch {
	case cfg.AgentKey != nil && cfg.AgentToken != "":
		return agentauth.Chain{agentauth.NewSigner(cfg.AgentKey), agentauth.Token(cfg.AgentToken)},
			"INFO  Agent requests are signed with DG_AGENT_KEY and still carry DG_AGENT_TOKEN for agents without the public key; remove the token once every agent has DG_AGENT_PUBLIC_KEY"
	case cfg.AgentKey != nil:
		return agentauth.NewSigner(cfg.AgentKey), "INFO  Agent requests are signed with DG_AGENT_KEY"
	case cfg.AgentToken != "":
		return agentauth.Token(cfg.AgentToken),
			"WARN  Agents authenticate requests with the shared DG_AGENT_TOKEN; switch to agent keys (dockgraph --agent-keygen, see the README). Token-only mode will be removed in a future release"
	case len(cfg.AgentPublicKeys) > 0:
		return nil, "WARN  DG_AGENT_PUBLIC_KEY is for agents; the server needs the private key (DG_AGENT_KEY). Agents disabled, stats and container logs limited to this node"
	default:
		return nil, "INFO  No agent credentials (DG_AGENT_KEY or DG_AGENT_TOKEN): agents disabled, stats and container logs limited to this node"
	}
}

// errAgentHasPrivateKey refuses to run an agent that was given the server's
// private key: agents must only ever hold the public key.
var errAgentHasPrivateKey = errors.New("DG_AGENT_KEY is the server's private key and must not be given to agents; agents take the public key (DG_AGENT_PUBLIC_KEY_FILE)")

// agentAuthNotice checks an agent's credentials and returns the line to log
// about them. Missing credentials are left to agent.New, which refuses them.
func agentAuthNotice(cfg Config) (string, error) {
	switch {
	case cfg.AgentKey != nil:
		return "", errAgentHasPrivateKey
	case len(cfg.AgentPublicKeys) > 0 && cfg.AgentToken != "":
		return "INFO  Accepting only requests signed by the server (DG_AGENT_PUBLIC_KEY); DG_AGENT_TOKEN is ignored", nil
	case len(cfg.AgentPublicKeys) > 0:
		return "INFO  Accepting only requests signed by the server (DG_AGENT_PUBLIC_KEY)", nil
	default:
		return "WARN  Accepting requests with the shared DG_AGENT_TOKEN; switch to agent keys (see the README). Token-only mode will be removed in a future release", nil
	}
}
