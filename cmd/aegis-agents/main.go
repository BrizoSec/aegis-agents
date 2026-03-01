package main

import (
	"log/slog"
	"os"

	"github.com/aegis/aegis-agents/config"
	"github.com/aegis/aegis-agents/internal/comms"
	"github.com/aegis/aegis-agents/internal/credentials"
	"github.com/aegis/aegis-agents/internal/factory"
	"github.com/aegis/aegis-agents/internal/lifecycle"
	"github.com/aegis/aegis-agents/internal/memory"
	"github.com/aegis/aegis-agents/internal/registry"
	"github.com/aegis/aegis-agents/internal/skills"
	"github.com/aegis/aegis-agents/pkg/types"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed", "error", err)
		os.Exit(1)
	}

	log.Info("starting aegis-agents",
		"component_id", cfg.ComponentID,
		"nats_url", cfg.NATSURL,
		"memory_addr", cfg.MemoryAddr,
	)

	// Wire dependencies.
	commsClient := comms.NewStubClient() // TODO: replace with NATS-backed client
	reg := registry.New()
	skillMgr := skills.New()
	credBroker := credentials.New(nil) // TODO: wire OpenBao secrets
	lifecycleMgr := lifecycle.New()    // TODO: wire Firecracker
	memClient := memory.New()          // TODO: replace with HTTP client to Memory Component

	seedSkills(skillMgr, log)

	f, err := factory.New(factory.Config{
		Registry:    reg,
		Skills:      skillMgr,
		Credentials: credBroker,
		Lifecycle:   lifecycleMgr,
		Memory:      memClient,
		Comms:       commsClient,
	})
	if err != nil {
		log.Error("factory init failed", "error", err)
		os.Exit(1)
	}

	// Subscribe to inbound task_spec messages from the Orchestrator.
	if err := commsClient.Subscribe("task_spec", func(msg *comms.Message) {
		// TODO: unmarshal Envelope → extract TaskSpec → call f.HandleTaskSpec
		log.Info("task_spec received", "bytes", len(msg.Data))
		_ = f // wired; handler body populated during integration
	}); err != nil {
		log.Error("subscribe task_spec failed", "error", err)
		os.Exit(1)
	}

	log.Info("aegis-agents ready")

	// Block forever. Replace with signal handling and graceful shutdown.
	select {}
}

// seedSkills registers the initial skill tree. In production this is loaded from
// the Memory Component or a config file at startup.
func seedSkills(mgr skills.Manager, log *slog.Logger) {
	domains := []*types.SkillNode{
		{
			Name:  "web",
			Level: "domain",
			Children: map[string]*types.SkillNode{
				"web.fetch": {
					Name:  "web.fetch",
					Level: "command",
					Spec: &types.SkillSpec{
						Parameters: map[string]types.ParameterDef{
							"url":    {Type: "string", Required: true},
							"method": {Type: "string", Required: false},
						},
					},
				},
			},
		},
		{
			Name:     "data",
			Level:    "domain",
			Children: map[string]*types.SkillNode{},
		},
		{
			Name:     "comms",
			Level:    "domain",
			Children: map[string]*types.SkillNode{},
		},
		{
			Name:     "storage",
			Level:    "domain",
			Children: map[string]*types.SkillNode{},
		},
	}

	for _, d := range domains {
		if err := mgr.RegisterDomain(d); err != nil {
			log.Warn("skill domain registration failed", "domain", d.Name, "error", err)
		}
	}
}
