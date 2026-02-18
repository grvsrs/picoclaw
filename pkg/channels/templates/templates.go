// Package templates — YAML-defined bot personality system.
//
// Templates describe a bot's personality, tools, schedule, and required
// parameters without touching Go code or config files. The dashboard reads
// templates and lets you instantiate bots from them.
//
// Template directories searched (in order):
//  1. ./templates/bots/    (relative to working directory)
//  2. ~/.picoclaw/templates/bots/
//  3. Embedded templates compiled into the binary
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ─────────────────────────────────────────────────────────────────────────────
// Template schema
// ─────────────────────────────────────────────────────────────────────────────

// BotTemplate is the YAML schema for a reusable bot personality.
type BotTemplate struct {
	// Identity
	Name        string `yaml:"name"`        // machine identifier (slug)
	DisplayName string `yaml:"display_name"` // human-readable
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	Author      string `yaml:"author,omitempty"`

	// Runtime
	Channel string   `yaml:"channel"` // telegram | discord | slack | webhook
	Soul    string   `yaml:"soul"`    // system prompt / personality definition
	Tools   []string `yaml:"tools"`   // tool names from the registry
	Cron    string   `yaml:"cron,omitempty"` // cron schedule (optional)

	// Parameters required to instantiate this template
	Params []TemplateParam `yaml:"params"`

	// Defaults applied to the instantiated bot config
	Defaults TemplateDefaults `yaml:"defaults"`

	// Source metadata (set by loader, not in YAML)
	SourceFile string `yaml:"-" json:"source_file,omitempty"`
	Builtin    bool   `yaml:"-" json:"builtin"`
}

// TemplateParam describes a required or optional instantiation parameter.
type TemplateParam struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default,omitempty"`
	Secret      bool   `yaml:"secret,omitempty"` // hint: mask in UI
}

// TemplateDefaults are values applied to the bot config that the user can
// override during instantiation.
type TemplateDefaults struct {
	AllowFrom []string `yaml:"allow_from"`
	MaxTokens int      `yaml:"max_tokens,omitempty"`
	Model     string   `yaml:"model,omitempty"`
}

// InstantiateRequest is the payload for creating a bot from a template.
type InstantiateRequest struct {
	Template   string            `json:"template"`             // template Name
	BotID      string            `json:"bot_id,omitempty"`     // override machine name
	Params     map[string]string `json:"params"`               // fills TemplateParam values
	AllowFrom  []string          `json:"allow_from,omitempty"` // override defaults
	AutoStart  bool              `json:"auto_start,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registry
// ─────────────────────────────────────────────────────────────────────────────

// Registry is a thread-safe store of loaded bot templates.
type Registry struct {
	mu        sync.RWMutex
	templates map[string]*BotTemplate
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		templates: make(map[string]*BotTemplate),
	}
}

// global singleton
var global = NewRegistry()

// Global returns the process-wide template registry.
func Global() *Registry { return global }

// Load reads all *.yaml files from dir and registers them.
// Errors in individual files are logged but don't abort loading.
func (r *Registry) Load(dir string) (int, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, []error{fmt.Errorf("cannot read template dir %s: %w", dir, err)}
	}

	loaded := 0
	var errs []error

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		tmpl, err := LoadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("load %s: %w", e.Name(), err))
			continue
		}
		r.Register(tmpl)
		loaded++
	}

	return loaded, errs
}

// LoadFile parses a single YAML template file.
func LoadFile(path string) (*BotTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tmpl BotTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}
	if tmpl.Name == "" {
		return nil, fmt.Errorf("template at %s has no 'name' field", path)
	}
	if tmpl.Channel == "" {
		return nil, fmt.Errorf("template '%s' has no 'channel' field", tmpl.Name)
	}
	tmpl.SourceFile = path
	return &tmpl, nil
}

// Register adds or replaces a template in the registry.
func (r *Registry) Register(tmpl *BotTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[tmpl.Name] = tmpl
}

// Get retrieves a template by name.
func (r *Registry) Get(name string) (*BotTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.templates[name]
	return t, ok
}

// List returns all registered templates, sorted by name.
func (r *Registry) List() []*BotTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*BotTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, t)
	}
	return out
}

// Count returns the number of registered templates.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.templates)
}

// ─────────────────────────────────────────────────────────────────────────────
// Validation
// ─────────────────────────────────────────────────────────────────────────────

// Validate checks that all required params are present in the provided map.
// Returns a list of missing required param names.
func (t *BotTemplate) Validate(params map[string]string) []string {
	var missing []string
	for _, p := range t.Params {
		if p.Required {
			v, ok := params[p.Name]
			if !ok || strings.TrimSpace(v) == "" {
				missing = append(missing, p.Name)
			}
		}
	}
	return missing
}

// ResolvedParams returns params merged with defaults (params take precedence).
func (t *BotTemplate) ResolvedParams(provided map[string]string) map[string]string {
	out := make(map[string]string, len(t.Params))
	for _, p := range t.Params {
		if p.Default != "" {
			out[p.Name] = p.Default
		}
	}
	for k, v := range provided {
		out[k] = v
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Auto-load from standard directories
// ─────────────────────────────────────────────────────────────────────────────

// LoadDefaults loads templates from all standard locations and returns a summary.
func LoadDefaults() (int, []string) {
	dirs := []string{
		"templates/bots",
		filepath.Join(os.Getenv("HOME"), ".picoclaw", "templates", "bots"),
	}

	total := 0
	var warnings []string

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		n, errs := global.Load(dir)
		total += n
		for _, e := range errs {
			warnings = append(warnings, e.Error())
		}
	}

	return total, warnings
}
