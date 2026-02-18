// Package app provides application services that orchestrate domain operations.
// These services sit between the API/UI layer and the domain layer,
// coordinating use cases across bounded contexts.
package app

import (
	"github.com/sipeed/picoclaw/pkg/domain"
	channeldomain "github.com/sipeed/picoclaw/pkg/domain/channel"
	agentdomain "github.com/sipeed/picoclaw/pkg/domain/agent"
	sessiondomain "github.com/sipeed/picoclaw/pkg/domain/session"
	skilldomain "github.com/sipeed/picoclaw/pkg/domain/skill"
	workflowdomain "github.com/sipeed/picoclaw/pkg/domain/workflow"
	providerdomain "github.com/sipeed/picoclaw/pkg/domain/provider"
)

// ---------------------------------------------------------------------------
// Application container — dependency injection root
// ---------------------------------------------------------------------------

// Container holds all application services and their dependencies.
// It acts as a composition root for dependency injection.
type Container struct {
	// Domain event bus
	EventBus domain.EventBus

	// Repositories
	Channels  channeldomain.Repository
	Agents    agentdomain.Repository
	Sessions  sessiondomain.Repository
	Skills    skilldomain.Repository
	Workflows workflowdomain.Repository
	Providers providerdomain.Repository

	// Domain services
	SkillRegistry skilldomain.Registry

	// Configuration
	WorkspacePath string
}

// NewContainer creates a fully wired application container.
func NewContainer(
	eventBus domain.EventBus,
	channels channeldomain.Repository,
	agents agentdomain.Repository,
	sessions sessiondomain.Repository,
	skills skilldomain.Repository,
	workflows workflowdomain.Repository,
	providers providerdomain.Repository,
	skillRegistry skilldomain.Registry,
	workspacePath string,
) *Container {
	return &Container{
		EventBus:      eventBus,
		Channels:      channels,
		Agents:        agents,
		Sessions:      sessions,
		Skills:        skills,
		Workflows:     workflows,
		Providers:     providers,
		SkillRegistry: skillRegistry,
		WorkspacePath: workspacePath,
	}
}

// PublishEvents dispatches pending events from an aggregate and clears them.
func (c *Container) PublishEvents(aggregate interface {
	PullEvents() []domain.Event
}) {
	events := aggregate.PullEvents()
	for _, event := range events {
		c.EventBus.Publish(event)
	}
}
