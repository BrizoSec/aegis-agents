// Package credentials is M5 — the Credential Broker. It implements the two-phase
// lazy credential delivery model: pre-authorize a permission set at agent spawn
// (Phase 1), then deliver individual secrets only when explicitly requested
// during skill invocation (Phase 2).
package credentials

import (
	"fmt"
	"sync"
)

// Broker is the interface for the two-phase credential lifecycle.
type Broker interface {
	// PreAuthorize registers the permission set for an agent at spawn time and
	// returns a scoped vault token. The token is stored internally; the agent
	// receives only a pointer, not the token itself.
	PreAuthorize(agentID string, permissionSet []string) (vaultToken string, err error)

	// GetCredential delivers a single credential value to an agent. It validates
	// that the requested key is within the pre-authorized permission set before
	// querying the vault.
	GetCredential(agentID, credentialKey string) (value string, err error)

	// Revoke invalidates the vault token and removes all pre-authorized state
	// for an agent. Called at agent termination.
	Revoke(agentID string) error
}

// agentAuth holds pre-authorized state for a single agent.
type agentAuth struct {
	vaultToken    string
	permissionSet map[string]struct{}
}

// stubBroker is the default implementation backed by in-process maps.
// Replace vault interactions with real OpenBao API calls when the Credential
// Vault is available.
type stubBroker struct {
	mu     sync.RWMutex
	agents map[string]*agentAuth

	// stubSecrets simulates the vault. In production this is replaced by
	// OpenBao API calls using the agent's scoped vault token.
	stubSecrets map[string]string
}

// New returns a Credential Broker backed by an in-process stub vault.
// Seed stubSecrets with test credentials as needed in tests.
func New(stubSecrets map[string]string) Broker {
	if stubSecrets == nil {
		stubSecrets = make(map[string]string)
	}
	return &stubBroker{
		agents:      make(map[string]*agentAuth),
		stubSecrets: stubSecrets,
	}
}

func (b *stubBroker) PreAuthorize(agentID string, permissionSet []string) (string, error) {
	if agentID == "" {
		return "", fmt.Errorf("credentials: agentID must not be empty")
	}
	if len(permissionSet) == 0 {
		return "", fmt.Errorf("credentials: permissionSet must not be empty")
	}

	token := "stub-token-" + agentID // deterministic for tests; production uses OpenBao

	perms := make(map[string]struct{}, len(permissionSet))
	for _, p := range permissionSet {
		perms[p] = struct{}{}
	}

	b.mu.Lock()
	b.agents[agentID] = &agentAuth{vaultToken: token, permissionSet: perms}
	b.mu.Unlock()

	return token, nil
}

func (b *stubBroker) GetCredential(agentID, credentialKey string) (string, error) {
	b.mu.RLock()
	auth, ok := b.agents[agentID]
	b.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("credentials: agent %q has no pre-authorized permission set", agentID)
	}
	if _, allowed := auth.permissionSet[credentialKey]; !allowed {
		return "", fmt.Errorf("credentials: key %q not in permission set for agent %q", credentialKey, agentID)
	}

	val, found := b.stubSecrets[credentialKey]
	if !found {
		return "", fmt.Errorf("credentials: key %q not found in vault", credentialKey)
	}
	return val, nil
}

func (b *stubBroker) Revoke(agentID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.agents[agentID]; !ok {
		return fmt.Errorf("credentials: agent %q not found", agentID)
	}
	delete(b.agents, agentID)
	return nil
}
