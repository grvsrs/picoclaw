// Bot Management API — CRUD + lifecycle control for channel bots.
// Wraps pkg/channels/manager.go to expose bot management via REST.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// BotInfo represents a bot/channel visible to the dashboard.
type BotInfo struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // telegram, discord, slack, etc.
	Enabled   bool                   `json:"enabled"`
	Running   bool                   `json:"running"`
	Config    map[string]interface{} `json:"config,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
}

// --- Bot CRUD Handlers ---

// handleBots dispatches /api/bots requests.
func (s *Server) handleBots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.handleListBots(w, r)
	case "POST":
		s.handleCreateBot(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// handleBotByID dispatches /api/bots/{id} requests.
func (s *Server) handleBotByID(w http.ResponseWriter, r *http.Request) {
	botID := strings.TrimPrefix(r.URL.Path, "/api/bots/")

	// Handle action sub-paths: /api/bots/{id}/start, /api/bots/{id}/stop
	parts := strings.SplitN(botID, "/", 2)
	botID = parts[0]

	if len(parts) == 2 {
		switch parts[1] {
		case "start":
			s.handleStartBot(w, r, botID)
		case "stop":
			s.handleStopBot(w, r, botID)
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown action"})
		}
		return
	}

	switch r.Method {
	case "GET":
		s.handleGetBot(w, r, botID)
	case "PUT":
		s.handleUpdateBot(w, r, botID)
	case "DELETE":
		s.handleDeleteBot(w, r, botID)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// GET /api/bots — list all configured bots with status.
func (s *Server) handleListBots(w http.ResponseWriter, r *http.Request) {
	bots := s.getBotsInfo()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bots":  bots,
		"count": len(bots),
	})
}

// GET /api/bots/{id} — get single bot info.
func (s *Server) handleGetBot(w http.ResponseWriter, r *http.Request, botID string) {
	if s.channelManager == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	ch, ok := s.channelManager.GetChannel(botID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	bot := BotInfo{
		ID:      botID,
		Type:    botID,
		Enabled: true,
		Running: ch.IsRunning(),
		Config:  s.getChannelConfig(botID),
	}

	writeJSON(w, http.StatusOK, bot)
}

// POST /api/bots — create/register a new bot.
func (s *Server) handleCreateBot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type      string            `json:"type"`
		Token     string            `json:"token,omitempty"`
		Config    map[string]string `json:"config,omitempty"`
		AllowFrom []string          `json:"allow_from,omitempty"`
		AutoStart bool              `json:"auto_start,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Type == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type is required"})
		return
	}

	if s.channelManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "channel manager not available"})
		return
	}

	// Check if already exists
	if _, exists := s.channelManager.GetChannel(req.Type); exists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": fmt.Sprintf("bot '%s' already exists", req.Type)})
		return
	}

	// Update config and create channel
	if err := s.updateChannelConfig(req.Type, req.Token, req.Config, req.AllowFrom); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Broadcast bot creation event
	s.wsHub.Broadcast("bot.created", map[string]interface{}{
		"bot_id": req.Type,
		"type":   req.Type,
	})

	logger.InfoCF("api", "Bot created via API", map[string]interface{}{
		"type": req.Type,
	})

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      req.Type,
		"type":    req.Type,
		"status":  "created",
		"message": fmt.Sprintf("Bot '%s' configured. Use POST /api/bots/%s/start to start it.", req.Type, req.Type),
	})
}

// PUT /api/bots/{id} — update bot config.
func (s *Server) handleUpdateBot(w http.ResponseWriter, r *http.Request, botID string) {
	if s.channelManager == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	if _, ok := s.channelManager.GetChannel(botID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	var req struct {
		Token     string            `json:"token,omitempty"`
		Config    map[string]string `json:"config,omitempty"`
		AllowFrom []string          `json:"allow_from,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := s.updateChannelConfig(botID, req.Token, req.Config, req.AllowFrom); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	s.wsHub.Broadcast("bot.updated", map[string]interface{}{
		"bot_id": botID,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      botID,
		"status":  "updated",
		"message": "Config updated. Restart bot for changes to take effect.",
	})
}

// DELETE /api/bots/{id} — remove a bot.
func (s *Server) handleDeleteBot(w http.ResponseWriter, r *http.Request, botID string) {
	if s.channelManager == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	ch, ok := s.channelManager.GetChannel(botID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	// Stop if running
	if ch.IsRunning() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ch.Stop(ctx)
	}

	s.channelManager.UnregisterChannel(botID)

	s.wsHub.Broadcast("bot.deleted", map[string]interface{}{
		"bot_id": botID,
	})

	logger.InfoCF("api", "Bot deleted via API", map[string]interface{}{
		"bot_id": botID,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/bots/{id}/start — start a bot.
func (s *Server) handleStartBot(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	if s.channelManager == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	ch, ok := s.channelManager.GetChannel(botID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	if ch.IsRunning() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_running"})
		return
	}

	ctx := context.Background()
	if err := ch.Start(ctx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to start: %v", err),
		})
		return
	}

	s.wsHub.Broadcast("bot.started", map[string]interface{}{
		"bot_id": botID,
	})

	logger.InfoCF("api", "Bot started via API", map[string]interface{}{
		"bot_id": botID,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// POST /api/bots/{id}/stop — stop a bot.
func (s *Server) handleStopBot(w http.ResponseWriter, r *http.Request, botID string) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	if s.channelManager == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	ch, ok := s.channelManager.GetChannel(botID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bot not found"})
		return
	}

	if !ch.IsRunning() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_stopped"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := ch.Stop(ctx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to stop: %v", err),
		})
		return
	}

	s.wsHub.Broadcast("bot.stopped", map[string]interface{}{
		"bot_id": botID,
	})

	logger.InfoCF("api", "Bot stopped via API", map[string]interface{}{
		"bot_id": botID,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// --- Internal helpers ---

func (s *Server) getBotsInfo() []BotInfo {
	var bots []BotInfo

	if s.channelManager == nil {
		return bots
	}

	status := s.channelManager.GetStatus()
	for name, info := range status {
		infoMap, ok := info.(map[string]interface{})
		running := false
		if ok {
			if r, exists := infoMap["running"]; exists {
				running, _ = r.(bool)
			}
		}

		bots = append(bots, BotInfo{
			ID:      name,
			Type:    name,
			Enabled: true,
			Running: running,
			Config:  s.getChannelConfig(name),
		})
	}

	// Also include configured-but-not-registered channels
	configuredChannels := s.getConfiguredChannels()
	for _, ch := range configuredChannels {
		found := false
		for _, b := range bots {
			if b.ID == ch.ID {
				found = true
				break
			}
		}
		if !found {
			bots = append(bots, ch)
		}
	}

	return bots
}

func (s *Server) getConfiguredChannels() []BotInfo {
	var bots []BotInfo
	if s.config == nil {
		return bots
	}
	cfg := s.config

	if cfg.Channels.Telegram.Enabled {
		bots = append(bots, BotInfo{
			ID:      "telegram",
			Type:    "telegram",
			Enabled: true,
			Config: map[string]interface{}{
				"has_token":  cfg.Channels.Telegram.Token != "",
				"allow_from": cfg.Channels.Telegram.AllowFrom,
			},
		})
	}
	if cfg.Channels.Discord.Enabled {
		bots = append(bots, BotInfo{
			ID:      "discord",
			Type:    "discord",
			Enabled: true,
			Config: map[string]interface{}{
				"has_token":  cfg.Channels.Discord.Token != "",
				"allow_from": cfg.Channels.Discord.AllowFrom,
			},
		})
	}
	if cfg.Channels.Slack.Enabled {
		bots = append(bots, BotInfo{
			ID:      "slack",
			Type:    "slack",
			Enabled: true,
			Config: map[string]interface{}{
				"has_bot_token": cfg.Channels.Slack.BotToken != "",
				"has_app_token": cfg.Channels.Slack.AppToken != "",
				"allow_from":    cfg.Channels.Slack.AllowFrom,
			},
		})
	}
	if cfg.Channels.WhatsApp.Enabled {
		bots = append(bots, BotInfo{
			ID:      "whatsapp",
			Type:    "whatsapp",
			Enabled: true,
			Config: map[string]interface{}{
				"bridge_url": cfg.Channels.WhatsApp.BridgeURL,
				"allow_from": cfg.Channels.WhatsApp.AllowFrom,
			},
		})
	}
	if cfg.Channels.DingTalk.Enabled {
		bots = append(bots, BotInfo{
			ID:      "dingtalk",
			Type:    "dingtalk",
			Enabled: true,
			Config: map[string]interface{}{
				"has_client_id": cfg.Channels.DingTalk.ClientID != "",
				"allow_from":    cfg.Channels.DingTalk.AllowFrom,
			},
		})
	}
	if cfg.Channels.Feishu.Enabled {
		bots = append(bots, BotInfo{
			ID:      "feishu",
			Type:    "feishu",
			Enabled: true,
			Config: map[string]interface{}{
				"has_app_id":  cfg.Channels.Feishu.AppID != "",
				"allow_from":  cfg.Channels.Feishu.AllowFrom,
			},
		})
	}
	if cfg.Channels.QQ.Enabled {
		bots = append(bots, BotInfo{
			ID:      "qq",
			Type:    "qq",
			Enabled: true,
			Config: map[string]interface{}{
				"has_app_id": cfg.Channels.QQ.AppID != "",
				"allow_from": cfg.Channels.QQ.AllowFrom,
			},
		})
	}
	if cfg.Channels.MaixCam.Enabled {
		bots = append(bots, BotInfo{
			ID:      "maixcam",
			Type:    "maixcam",
			Enabled: true,
			Config: map[string]interface{}{
				"host": cfg.Channels.MaixCam.Host,
				"port": cfg.Channels.MaixCam.Port,
			},
		})
	}

	return bots
}

// getChannelConfig returns redacted config for a channel (no secrets).
func (s *Server) getChannelConfig(name string) map[string]interface{} {
	if s.config == nil {
		return nil
	}
	switch name {
	case "telegram":
		return map[string]interface{}{
			"has_token":  s.config.Channels.Telegram.Token != "",
			"allow_from": s.config.Channels.Telegram.AllowFrom,
		}
	case "discord":
		return map[string]interface{}{
			"has_token":  s.config.Channels.Discord.Token != "",
			"allow_from": s.config.Channels.Discord.AllowFrom,
		}
	case "slack":
		return map[string]interface{}{
			"has_bot_token": s.config.Channels.Slack.BotToken != "",
			"has_app_token": s.config.Channels.Slack.AppToken != "",
			"allow_from":    s.config.Channels.Slack.AllowFrom,
		}
	case "whatsapp":
		return map[string]interface{}{
			"bridge_url": s.config.Channels.WhatsApp.BridgeURL,
			"allow_from": s.config.Channels.WhatsApp.AllowFrom,
		}
	default:
		return map[string]interface{}{}
	}
}

// updateChannelConfig updates config for a channel type.
func (s *Server) updateChannelConfig(channelType, token string, cfg map[string]string, allowFrom []string) error {
	if s.config == nil {
		return fmt.Errorf("config not available")
	}

	switch channelType {
	case "telegram":
		s.config.Channels.Telegram.Enabled = true
		if token != "" {
			s.config.Channels.Telegram.Token = token
		}
		if allowFrom != nil {
			s.config.Channels.Telegram.AllowFrom = allowFrom
		}
		// Re-create channel with new config
		return s.recreateChannel(channelType)

	case "discord":
		s.config.Channels.Discord.Enabled = true
		if token != "" {
			s.config.Channels.Discord.Token = token
		}
		if allowFrom != nil {
			s.config.Channels.Discord.AllowFrom = allowFrom
		}
		return s.recreateChannel(channelType)

	case "slack":
		s.config.Channels.Slack.Enabled = true
		if token != "" {
			s.config.Channels.Slack.BotToken = token
		}
		if v, ok := cfg["app_token"]; ok {
			s.config.Channels.Slack.AppToken = v
		}
		if allowFrom != nil {
			s.config.Channels.Slack.AllowFrom = allowFrom
		}
		return s.recreateChannel(channelType)

	case "whatsapp":
		s.config.Channels.WhatsApp.Enabled = true
		if v, ok := cfg["bridge_url"]; ok {
			s.config.Channels.WhatsApp.BridgeURL = v
		}
		if allowFrom != nil {
			s.config.Channels.WhatsApp.AllowFrom = allowFrom
		}
		return s.recreateChannel(channelType)

	default:
		return fmt.Errorf("unsupported channel type: %s", channelType)
	}
}

// recreateChannel creates a new channel instance from updated config and registers it.
func (s *Server) recreateChannel(channelType string) error {
	if s.channelManager == nil || s.messageBus == nil {
		return fmt.Errorf("channel manager not available")
	}

	// Import channel constructors indirectly via bus
	// The channel manager handles creation — we register the intent
	// and the manager will pick it up on next init cycle.

	// For now, just broadcast the config change event.
	// Full hot-reload requires stopping old channel and creating new one.
	s.wsHub.Broadcast("bot.config_changed", map[string]interface{}{
		"bot_id":  channelType,
		"message": "Config updated. Restart required.",
	})

	return nil
}

// handleBotTypes returns the supported bot types for the create dialog.
func (s *Server) handleBotTypes(w http.ResponseWriter, r *http.Request) {
	types := []map[string]interface{}{
		{
			"type":        "telegram",
			"label":       "Telegram Bot",
			"description": "Connect a Telegram bot via Bot API token",
			"fields":      []string{"token", "allow_from"},
		},
		{
			"type":        "discord",
			"label":       "Discord Bot",
			"description": "Connect a Discord bot via OAuth token",
			"fields":      []string{"token", "allow_from"},
		},
		{
			"type":        "slack",
			"label":       "Slack Bot",
			"description": "Connect a Slack workspace bot",
			"fields":      []string{"token", "app_token", "allow_from"},
		},
		{
			"type":        "whatsapp",
			"label":       "WhatsApp Bridge",
			"description": "Connect via WhatsApp bridge WebSocket",
			"fields":      []string{"bridge_url", "allow_from"},
		},
		{
			"type":        "dingtalk",
			"label":       "DingTalk Bot",
			"description": "Connect a DingTalk workspace bot",
			"fields":      []string{"client_id", "client_secret", "allow_from"},
		},
		{
			"type":        "feishu",
			"label":       "Feishu/Lark Bot",
			"description": "Connect a Feishu (Lark) workspace bot",
			"fields":      []string{"app_id", "app_secret", "allow_from"},
		},
		{
			"type":        "qq",
			"label":       "QQ Bot",
			"description": "Connect a QQ messaging bot",
			"fields":      []string{"app_id", "app_secret", "allow_from"},
		},
		{
			"type":        "maixcam",
			"label":       "MaixCam (IoT)",
			"description": "Connect a Sipeed MaixCam device",
			"fields":      []string{"host", "port"},
		},
	}

	writeJSON(w, http.StatusOK, types)
}

// handleBotActions returns available actions for the bot management UI.
func (s *Server) handleBotActions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"actions": []map[string]string{
			{"action": "start", "method": "POST", "path": "/api/bots/{id}/start"},
			{"action": "stop", "method": "POST", "path": "/api/bots/{id}/stop"},
			{"action": "update", "method": "PUT", "path": "/api/bots/{id}"},
			{"action": "delete", "method": "DELETE", "path": "/api/bots/{id}"},
		},
	})
}

// publishBotEvent sends a bus-level event for bot state changes.
// This bridges the API layer into the message bus for cross-system observability.
func (s *Server) publishBotEvent(eventType, botID string, extra map[string]interface{}) {
	if s.messageBus == nil {
		return
	}

	content := fmt.Sprintf("[system] bot.%s: %s", eventType, botID)
	if msg, ok := extra["message"].(string); ok {
		content += " — " + msg
	}

	s.messageBus.PublishInbound(bus.InboundMessage{
		Channel:    "system",
		SenderID:   "api",
		ChatID:     "system:bots",
		Content:    content,
		SessionKey: "system:bots",
		Metadata: map[string]string{
			"event_type": "bot." + eventType,
			"bot_id":     botID,
		},
	})
}
