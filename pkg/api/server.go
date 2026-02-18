// PicoClaw - Web Dashboard API Server
// Serves REST endpoints + WebSocket for live events + embedded static UI
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/channels/templates"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cron"
	"github.com/sipeed/picoclaw/pkg/logger"
)

// Server is the HTTP API server for the PicoClaw dashboard.
type Server struct {
	config         *config.Config
	agentLoop      *agent.AgentLoop
	channelManager *channels.Manager
	cronService    *cron.CronService
	messageBus     *bus.MessageBus
	wsHub          *WSHub
	eventBridge    *EventBridge
	startTime      time.Time
	server         *http.Server
	webFS          fs.FS
	mu             sync.RWMutex
}

// NewServer creates a new API server instance.
func NewServer(
	cfg *config.Config,
	agentLoop *agent.AgentLoop,
	channelMgr *channels.Manager,
	cronSvc *cron.CronService,
	msgBus *bus.MessageBus,
	webFS fs.FS,
) *Server {
	// --- Secure-by-default: auto-generate API key if none is configured ---
	// Follows the Jupyter pattern: random key per session, printed once at startup.
	// Set gateway.api_key in config.json or PICOCLAW_API_KEY env var for a persistent key.
	if cfg.Gateway.APIKey == "" {
		raw := make([]byte, 24)
		if _, err := rand.Read(raw); err == nil {
			cfg.Gateway.APIKey = hex.EncodeToString(raw)
			fmt.Println()
			fmt.Println("╔══════════════════════════════════════════════════════╗")
			fmt.Println("║          PICOCLAW API KEY (session token)            ║")
			fmt.Printf( "║  %-52s  ║\n", cfg.Gateway.APIKey)
			fmt.Println("║  Set gateway.api_key in config.json to make          ║")
			fmt.Println("║  this permanent. Rotate it any time.                 ║")
			fmt.Println("╚══════════════════════════════════════════════════════╝")
			fmt.Println()
		}
	}
	s := &Server{
		config:         cfg,
		agentLoop:      agentLoop,
		channelManager: channelMgr,
		cronService:    cronSvc,
		messageBus:     msgBus,
		startTime:      time.Now(),
		webFS:          webFS,
	}
	s.wsHub = NewWSHub(s)
	s.eventBridge = NewEventBridge(msgBus, s.wsHub)

	// Load bot templates from standard locations at startup
	n, warns := templates.LoadDefaults()
	logger.InfoCF("api", "Bot templates loaded", map[string]interface{}{
		"count": n,
	})
	for _, w := range warns {
		logger.ErrorCF("api", "Template load warning", map[string]interface{}{"warn": w})
	}

	return s
}

// Start begins listening on the configured host:port.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/system/status", s.handleSystemStatus)
	mux.HandleFunc("/api/system/info", s.handleSystemInfo)

	mux.HandleFunc("/api/channels", s.handleChannels)

	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/", s.handleSessionDetail)

	mux.HandleFunc("/api/tools", s.handleTools)

	mux.HandleFunc("/api/cron/jobs", s.handleCronJobs)
	mux.HandleFunc("/api/cron/status", s.handleCronStatus)

	mux.HandleFunc("/api/agent/chat", s.handleAgentChat)
	mux.HandleFunc("/api/agent/status", s.handleAgentStatus)

	// Bot management API
	mux.HandleFunc("/api/bots", s.handleBots)
	mux.HandleFunc("/api/bots/from-template", s.handleCreateBotFromTemplate)
	mux.HandleFunc("/api/bots/", s.handleBotByID)
	mux.HandleFunc("/api/bot-templates", s.handleListBotTemplates)
	mux.HandleFunc("/api/bot-types", s.handleBotTypes)

	// Kanban proxy (forwards to Python kanban server)
	mux.HandleFunc("/api/kanban/", s.handleKanbanProxy)

	// Native Go task API (authoritative kanban spine)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskByID)

	// VSCode extension API
	mux.HandleFunc("/api/vscode/", s.handleVSCode)

	// Webhook ingestion (local programs → picoclaw)
	mux.HandleFunc("/api/webhook/{source}", s.handleWebhook)

	// Workflow event ingestion (ide-monitor → picoclaw)
	mux.HandleFunc("/api/events", s.handleWorkflowEvent)

	// WebSocket for live events
	mux.HandleFunc("/api/ws", s.wsHub.HandleWebSocket)

	// Serve embedded static files for the dashboard UI
	mux.HandleFunc("/", s.handleStaticFiles)

	addr := fmt.Sprintf("%s:%d", s.config.Gateway.Host, s.config.Gateway.Port)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(authMiddleware(s.config.Gateway.APIKey, mux)),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.InfoCF("api", "Dashboard API server starting", map[string]interface{}{
		"addr": addr,
	})

	go s.wsHub.Run(ctx)
	go s.eventBridge.Run(ctx)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.ErrorCF("api", "Server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// --- Middleware ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if the origin is a trusted localhost address.
func isAllowedOrigin(origin string) bool {
	for _, prefix := range []string{"http://localhost", "http://127.0.0.1", "https://localhost", "https://127.0.0.1"} {
		if strings.HasPrefix(origin, prefix) {
			return true
		}
	}
	return false
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startTime)

	channelStatus := make(map[string]interface{})
	if s.channelManager != nil {
		channelStatus = s.channelManager.GetStatus()
	}

	cronStatus := make(map[string]interface{})
	if s.cronService != nil {
		cronStatus = s.cronService.Status()
	}

	agentRunning := false
	model := ""
	toolCount := 0
	var toolNames []string
	if s.agentLoop != nil {
		agentRunning = s.agentLoop.IsRunning()
		model = s.agentLoop.GetModel()
		toolNames = s.agentLoop.GetToolRegistry().List()
		toolCount = len(toolNames)
	}

	sessionCount := 0
	if s.agentLoop != nil && s.agentLoop.GetSessionManager() != nil {
		sessionCount = len(s.agentLoop.GetSessionManager().ListSessions())
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"uptime_seconds": int(uptime.Seconds()),
		"uptime_human":   formatDuration(uptime),
		"agent": map[string]interface{}{
			"running":    agentRunning,
			"model":      model,
			"tools":      toolCount,
			"tool_names": toolNames,
		},
		"channels": channelStatus,
		"cron":     cronStatus,
		"sessions": sessionCount,
	})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	hostname, _ := os.Hostname()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"hostname":    hostname,
		"go_version":  runtime.Version(),
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"cpus":        runtime.NumCPU(),
		"goroutines":  runtime.NumGoroutine(),
		"memory_mb":   float64(m.Alloc) / 1024 / 1024,
		"sys_mb":      float64(m.Sys) / 1024 / 1024,
		"gc_cycles":   m.NumGC,
		"workspace":   s.config.WorkspacePath(),
		"gateway_host": s.config.Gateway.Host,
		"gateway_port": s.config.Gateway.Port,
	})
}

func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	if s.channelManager == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, s.channelManager.GetStatus())
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if s.agentLoop == nil || s.agentLoop.GetSessionManager() == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	sessions := s.agentLoop.GetSessionManager().ListSessions()

	// Enrich with message counts
	result := make([]map[string]interface{}, 0, len(sessions))
	for _, sess := range sessions {
		msgCount := s.agentLoop.GetSessionManager().MessageCount(sess.Key)
		result = append(result, map[string]interface{}{
			"key":            sess.Key,
			"summary":        sess.Summary,
			"message_count":  msgCount,
			"created":        sess.Created,
			"updated":        sess.Updated,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	// Extract session key from URL: /api/sessions/{key}
	key := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session key required"})
		return
	}

	if s.agentLoop == nil || s.agentLoop.GetSessionManager() == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	if r.Method == "DELETE" {
		ok := s.agentLoop.GetSessionManager().DeleteSession(key)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}

	session, ok := s.agentLoop.GetSessionManager().GetSession(key)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if s.agentLoop == nil || s.agentLoop.GetToolRegistry() == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	definitions := s.agentLoop.GetToolRegistry().GetDefinitions()
	writeJSON(w, http.StatusOK, definitions)
}

func (s *Server) handleCronJobs(w http.ResponseWriter, r *http.Request) {
	if s.cronService == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	jobs := s.cronService.ListJobs(true)
	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleCronStatus(w http.ResponseWriter, r *http.Request) {
	if s.cronService == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	writeJSON(w, http.StatusOK, s.cronService.Status())
}

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req struct {
		Message string `json:"message"`
		Session string `json:"session"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message required"})
		return
	}

	sessionKey := req.Session
	if sessionKey == "" {
		sessionKey = "web:dashboard"
	}

	if s.agentLoop == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent not available"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	response, err := s.agentLoop.ProcessDirectWithChannel(ctx, req.Message, sessionKey, "web", "dashboard")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"response": response,
		"session":  sessionKey,
	})
}

func (s *Server) handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	if s.agentLoop == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"running": false})
		return
	}

	info := s.agentLoop.GetStartupInfo()
	info["running"] = s.agentLoop.IsRunning()
	info["model"] = s.agentLoop.GetModel()
	info["workspace"] = s.agentLoop.GetWorkspace()

	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleStaticFiles(w http.ResponseWriter, r *http.Request) {
	var staticFS fs.FS

	if s.webFS != nil {
		staticFS = s.webFS
	} else {
		// Fallback: serve from local web/dist directory
		staticFS = os.DirFS("web/dist")
	}

	// For SPA routing: if file doesn't exist, serve index.html
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Try to open the file
	f, err := staticFS.Open(strings.TrimPrefix(path, "/"))
	if err != nil {
		// Serve index.html for SPA client-side routing
		r.URL.Path = "/index.html"
	} else {
		f.Close()
	}

	http.FileServer(http.FS(staticFS)).ServeHTTP(w, r)
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
