// Bot template API — serves YAML-defined bot personalities and handles
// template-based bot instantiation via POST /api/bots/from-template.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/channels/templates"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// GET /api/bot-templates — list all available bot templates.
func (s *Server) handleListBotTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	reg := templates.Global()
	list := reg.List()

	// Build a clean representation — never expose secret default values
	type templateView struct {
		Name        string                    `json:"name"`
		DisplayName string                    `json:"display_name"`
		Description string                    `json:"description"`
		Version     string                    `json:"version"`
		Channel     string                    `json:"channel"`
		Soul        string                    `json:"soul"`
		Tools       []string                  `json:"tools"`
		Cron        string                    `json:"cron,omitempty"`
		Params      []templates.TemplateParam `json:"params"`
		Builtin     bool                      `json:"builtin"`
	}

	views := make([]templateView, 0, len(list))
	for _, t := range list {
		views = append(views, templateView{
			Name:        t.Name,
			DisplayName: t.DisplayName,
			Description: t.Description,
			Version:     t.Version,
			Channel:     t.Channel,
			Soul:        t.Soul,
			Tools:       t.Tools,
			Cron:        t.Cron,
			Params:      t.Params,
			Builtin:     t.Builtin,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"templates": views,
		"count":     len(views),
	})
}

// POST /api/bots/from-template — instantiate a bot from a named template.
//
// Request body:
//
//	{
//	    "template": "telegram-assistant",
//	    "bot_id": "my-assistant",        // optional override
//	    "params": {
//	        "token": "12345:ABC...",
//	        "allow_from": "123456789"
//	    },
//	    "allow_from": ["123456789"],     // optional override
//	    "auto_start": true
//	}
func (s *Server) handleCreateBotFromTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req templates.InstantiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Template == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "template name is required"})
		return
	}

	// Look up the template
	reg := templates.Global()
	tmpl, ok := reg.Get(req.Template)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("template '%s' not found", req.Template),
		})
		return
	}

	// Validate required params
	if missing := tmpl.Validate(req.Params); len(missing) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "missing required parameters",
			"missing": missing,
		})
		return
	}

	// Resolve bot ID: explicit override → template name → slug
	botID := req.BotID
	if botID == "" {
		botID = tmpl.Name
	}
	botID = strings.ToLower(strings.ReplaceAll(botID, " ", "-"))

	if s.channelManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "channel manager not available"})
		return
	}

	// Check for existing bot with this ID
	if _, exists := s.channelManager.GetChannel(botID); exists {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("a bot with id '%s' already exists", botID),
		})
		return
	}

	// Resolve params (merge defaults + provided)
	resolved := tmpl.ResolvedParams(req.Params)

	// Extract standard fields from resolved params
	token := resolved["token"]
	allowFrom := req.AllowFrom
	if len(allowFrom) == 0 && resolved["allow_from"] != "" {
		// Parse comma-separated allow_from from params
		for _, id := range strings.Split(resolved["allow_from"], ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				allowFrom = append(allowFrom, id)
			}
		}
	}
	if len(allowFrom) == 0 {
		allowFrom = tmpl.Defaults.AllowFrom
	}

	// Build extended config from remaining resolved params + template metadata
	extraConfig := map[string]string{
		"soul":         tmpl.Soul,
		"template":     tmpl.Name,
		"display_name": tmpl.DisplayName,
	}
	if tmpl.Cron != "" {
		extraConfig["cron"] = tmpl.Cron
	}
	for k, v := range resolved {
		if k != "token" && k != "allow_from" {
			extraConfig[k] = v
		}
	}

	// Delegate to the existing updateChannelConfig mechanism
	if err := s.updateChannelConfig(tmpl.Channel, token, extraConfig, allowFrom); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	logger.InfoCF("api", "Bot instantiated from template", map[string]interface{}{
		"bot_id":   botID,
		"template": tmpl.Name,
		"channel":  tmpl.Channel,
	})

	// Broadcast creation event
	s.wsHub.Broadcast("bot.created", map[string]interface{}{
		"bot_id":   botID,
		"template": tmpl.Name,
		"channel":  tmpl.Channel,
		"source":   "template",
	})

	resp := map[string]interface{}{
		"id":       botID,
		"template": tmpl.Name,
		"channel":  tmpl.Channel,
		"status":   "created",
		"message":  fmt.Sprintf("Bot '%s' created from template '%s'.", botID, tmpl.Name),
	}
	if req.AutoStart {
		resp["message"] = fmt.Sprintf("Bot '%s' created from template '%s'. Use POST /api/bots/%s/start to start it.", botID, tmpl.Name, botID)
	}

	writeJSON(w, http.StatusCreated, resp)
}
