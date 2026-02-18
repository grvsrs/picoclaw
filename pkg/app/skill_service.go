package app

import (
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/domain"
	skilldomain "github.com/sipeed/picoclaw/pkg/domain/skill"
)

// ---------------------------------------------------------------------------
// Skill application service
// ---------------------------------------------------------------------------

// SkillService orchestrates skill ecosystem use cases.
type SkillService struct {
	repo     skilldomain.Repository
	registry skilldomain.Registry
	eventBus domain.EventBus
	factory  skilldomain.Factory
}

// NewSkillService creates a new skill application service.
func NewSkillService(repo skilldomain.Repository, registry skilldomain.Registry, eventBus domain.EventBus) *SkillService {
	return &SkillService{
		repo:     repo,
		registry: registry,
		eventBus: eventBus,
	}
}

// RegisterSkill creates, persists, and registers a new skill.
func (s *SkillService) RegisterSkill(name, version, description string, category skilldomain.SkillCategory, source domain.SkillSource, spec skilldomain.SkillSpec) (*skilldomain.Skill, error) {
	// Check for duplicate
	if existing, _ := s.repo.FindByName(name); existing != nil {
		return nil, fmt.Errorf("skill '%s' already exists", name)
	}

	skill, err := s.factory.CreateSkill(name, version, description, category, source, spec)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Save(skill); err != nil {
		return nil, fmt.Errorf("save skill: %w", err)
	}

	if err := s.registry.Register(skill); err != nil {
		return nil, fmt.Errorf("register skill: %w", err)
	}

	s.publishEvents(skill)
	return skill, nil
}

// InstallSkill marks a skill as installed at a path.
func (s *SkillService) InstallSkill(id domain.EntityID, path string) error {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	skill.Install(path)
	if err := s.repo.Save(skill); err != nil {
		return err
	}

	s.publishEvents(skill)
	return nil
}

// UninstallSkill removes a skill.
func (s *SkillService) UninstallSkill(id domain.EntityID) error {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	skill.Uninstall()
	s.registry.Unregister(skill.Name)

	if err := s.repo.Save(skill); err != nil {
		return err
	}

	s.publishEvents(skill)
	return nil
}

// EnableSkill activates a skill.
func (s *SkillService) EnableSkill(id domain.EntityID) error {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	skill.Enable()
	return s.repo.Save(skill)
}

// DisableSkill deactivates a skill.
func (s *SkillService) DisableSkill(id domain.EntityID) error {
	skill, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}

	skill.Disable()
	return s.repo.Save(skill)
}

// SearchSkills finds skills matching a query, category, and/or tags.
func (s *SkillService) SearchSkills(query string, category string, tags []string) ([]*skilldomain.Skill, error) {
	if query != "" {
		return s.repo.Search(query)
	}

	if category != "" {
		return s.repo.FindByCategory(skilldomain.SkillCategory(category))
	}

	if len(tags) > 0 {
		domainTags := make(domain.Tags, len(tags))
		for i, t := range tags {
			domainTags[i] = domain.Tag(t)
		}
		return s.repo.FindByTags(domainTags)
	}

	return s.repo.FindAll()
}

// GetSkill retrieves a skill by ID.
func (s *SkillService) GetSkill(id domain.EntityID) (*skilldomain.Skill, error) {
	return s.repo.FindByID(id)
}

// GetSkillByName retrieves a skill by name.
func (s *SkillService) GetSkillByName(name string) (*skilldomain.Skill, error) {
	return s.repo.FindByName(name)
}

// ListSkills returns all skills.
func (s *SkillService) ListSkills() ([]*skilldomain.Skill, error) {
	return s.repo.FindAll()
}

// ListByCategory returns skills in a category.
func (s *SkillService) ListByCategory(category string) ([]*skilldomain.Skill, error) {
	return s.repo.FindByCategory(skilldomain.SkillCategory(category))
}

// GetRegistryStats returns skill registry statistics.
func (s *SkillService) GetRegistryStats() map[string]interface{} {
	skills, _ := s.repo.FindAll()

	categories := make(map[string]int)
	sources := make(map[string]int)
	enabledCount := 0
	totalExecutions := int64(0)

	for _, sk := range skills {
		categories[string(sk.Category)]++
		sources[string(sk.Source)]++
		if sk.Enabled {
			enabledCount++
		}
		totalExecutions += sk.Metrics.ExecutionCount
	}

	return map[string]interface{}{
		"total":            len(skills),
		"enabled":          enabledCount,
		"categories":       categories,
		"sources":          sources,
		"total_executions": totalExecutions,
	}
}

// RecordExecution tracks a skill execution result.
func (s *SkillService) RecordExecution(name string, durationMS int64, err error) {
	skill, findErr := s.repo.FindByName(name)
	if findErr != nil {
		return
	}

	if err != nil {
		skill.RecordError(err.Error())
	} else {
		skill.RecordExecution(durationMS)
	}

	s.repo.Save(skill)

	eventType := domain.EventSkillExecuted
	eventData := map[string]interface{}{
		"skill":       name,
		"duration_ms": durationMS,
		"success":     err == nil,
	}
	if err != nil {
		eventType = domain.EventSkillError
		eventData["error"] = err.Error()
	}
	s.eventBus.Publish(domain.NewEvent(eventType, skill.ID(), eventData))
}

// ValidateDependencies checks that all dependencies of a skill are available.
func (s *SkillService) ValidateDependencies(skillName string) []string {
	skill, err := s.repo.FindByName(skillName)
	if err != nil {
		return []string{fmt.Sprintf("skill '%s' not found", skillName)}
	}

	var missing []string
	for _, dep := range skill.Dependencies {
		if _, err := s.repo.FindByName(dep.SkillName); err != nil {
			if dep.Required {
				missing = append(missing, fmt.Sprintf("required: %s", dep.SkillName))
			} else {
				missing = append(missing, fmt.Sprintf("optional: %s", dep.SkillName))
			}
		}
	}
	return missing
}

// GenerateSkillSummary produces a human-readable summary of all skills.
func (s *SkillService) GenerateSkillSummary() string {
	skills, _ := s.repo.FindAll()
	if len(skills) == 0 {
		return "No skills registered."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Skill Registry: %d skills\n\n", len(skills)))

	categories := make(map[skilldomain.SkillCategory][]*skilldomain.Skill)
	for _, sk := range skills {
		categories[sk.Category] = append(categories[sk.Category], sk)
	}

	for cat, catSkills := range categories {
		sb.WriteString(fmt.Sprintf("## %s\n", string(cat)))
		for _, sk := range catSkills {
			status := "✓"
			if !sk.Enabled {
				status = "✗"
			}
			sb.WriteString(fmt.Sprintf("  %s %s v%s — %s\n", status, sk.Name, sk.Version, sk.Description))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (s *SkillService) publishEvents(skill *skilldomain.Skill) {
	events := skill.PullEvents()
	for _, event := range events {
		s.eventBus.Publish(event)
	}
}
