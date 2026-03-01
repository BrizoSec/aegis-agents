package config

import (
	"fmt"
	"os"
)

// Config holds all external endpoint configuration loaded from the environment.
type Config struct {
	// NATS JetStream URL (Communications Component)
	NATSURL string

	// OpenBao API address (Credential Vault)
	OpenBaoAddr string

	// Memory Component HTTP endpoint
	MemoryAddr string

	// Component identity published in message envelopes
	ComponentID string
}

// Load reads configuration from environment variables and returns a validated Config.
// All fields are required; Load returns an error if any are missing.
func Load() (*Config, error) {
	c := &Config{
		NATSURL:     os.Getenv("AEGIS_NATS_URL"),
		OpenBaoAddr: os.Getenv("AEGIS_OPENBAO_ADDR"),
		MemoryAddr:  os.Getenv("AEGIS_MEMORY_ADDR"),
		ComponentID: os.Getenv("AEGIS_COMPONENT_ID"),
	}

	if c.NATSURL == "" {
		return nil, fmt.Errorf("config: AEGIS_NATS_URL is required")
	}
	if c.OpenBaoAddr == "" {
		return nil, fmt.Errorf("config: AEGIS_OPENBAO_ADDR is required")
	}
	if c.MemoryAddr == "" {
		return nil, fmt.Errorf("config: AEGIS_MEMORY_ADDR is required")
	}
	if c.ComponentID == "" {
		c.ComponentID = "aegis-agents"
	}

	return c, nil
}
