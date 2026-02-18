// Kanban proxy — routes /api/kanban/* through the Go backend to the Python
// kanban server, providing single-origin access and unified auth.
package api

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// handleKanbanProxy forwards requests to the Python kanban server.
// The Python server must be running separately (kanban_server.py).
//
// Mapping:
//
//	GET  /api/kanban/board    → GET  <kanban>/api/board
//	GET  /api/kanban/cards    → GET  <kanban>/api/cards
//	POST /api/kanban/cards    → POST <kanban>/api/cards
//	PUT  /api/kanban/cards/X  → PUT  <kanban>/api/cards/X
//	POST /api/kanban/cards/X/transition → POST <kanban>/api/cards/X/transition
//	GET  /api/kanban/stats    → GET  <kanban>/api/stats
//	GET  /api/kanban/categories → GET <kanban>/api/categories
//	POST /api/kanban/categorize → POST <kanban>/api/categorize
//	POST /api/kanban/categorize/card/X → POST <kanban>/api/categorize/card/X
func (s *Server) handleKanbanProxy(w http.ResponseWriter, r *http.Request) {
	// Strip the /api/kanban prefix to get the Python API path
	targetPath := strings.TrimPrefix(r.URL.Path, "/api/kanban")
	if targetPath == "" {
		targetPath = "/"
	}

	// Get kanban server URL from config
	kanbanURL := s.config.Integrations.KanbanServerURL
	if kanbanURL == "" {
		kanbanURL = "http://127.0.0.1:5000"
	}

	proxyURL := kanbanURL + "/api" + targetPath
	if r.URL.RawQuery != "" {
		proxyURL += "?" + r.URL.RawQuery
	}

	// Create proxy request
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL, r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "failed to create proxy request",
		})
		return
	}

	// Forward relevant headers
	proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		proxyReq.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		logger.WarnCF("kanban-proxy", "Kanban server unreachable", map[string]interface{}{
			"url":   proxyURL,
			"error": err.Error(),
		})
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":   "kanban server unreachable",
			"details": "Ensure kanban_server.py is running on " + kanbanURL,
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
