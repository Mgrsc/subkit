package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"subkit/internal/config"
	"subkit/internal/converter"
	"subkit/internal/logger"

	"github.com/google/uuid"
)

const (
	contentTypeJSON = "application/json"
	contentTypeYAML = "application/yaml"
)

type Server struct {
	assembler        *config.Assembler
	extractor        *converter.Extractor
	cache            map[string][]byte
	cacheMu          sync.RWMutex
	subscriptionName string
}

type ConvertRequest struct {
	Input              string   `json:"input"`
	URIs               []string `json:"uris"`
	CustomRequirements string   `json:"custom_requirements"`
}

type ConvertResponse struct {
	ConfigID string `json:"config_id"`
	URL      string `json:"url"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ExtractNodesRequest struct {
	Input string `json:"input"`
}

type NodeWithURI struct {
	Node *converter.ProxyNode `json:"node"`
	URI  string               `json:"uri"`
}

type ExtractNodesResponse struct {
	Nodes []*NodeWithURI `json:"nodes"`
	Count int            `json:"count"`
}

func NewServer() (*Server, error) {
	assembler, err := config.NewAssembler()
	if err != nil {
		return nil, fmt.Errorf("create assembler failed: %w", err)
	}

	if err := assembler.LoadRuleLists(); err != nil {
		logger.Info("Load rule lists failed: %v", err)
	}

	subscriptionName := os.Getenv("SUBKIT_SUBSCRIBE_NAME")
	if subscriptionName == "" {
		subscriptionName = "Subkit Mihomo"
	}

	return &Server{
		assembler:        assembler,
		extractor:        converter.NewExtractor(),
		cache:            make(map[string][]byte),
		subscriptionName: subscriptionName,
	}, nil
}


func (s *Server) ReloadRuleLists() {
	logger.Info("[Server] Reloading rule lists...")
	if err := s.assembler.LoadRuleLists(); err != nil {
		logger.Warn("[Server] Failed to reload rule lists: %v", err)
	} else {
		logger.Info("[Server] Rule lists reloaded successfully")
	}
}

func (s *Server) buildSubscriptionURL(r *http.Request, configID string) string {
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}

	subURL := fmt.Sprintf("http://%s/subscribe/%s", host, configID)
	if s.subscriptionName != "" {
		subURL = fmt.Sprintf("%s?name=%s", subURL, url.QueryEscape(s.subscriptionName))
	}
	return subURL
}

func (s *Server) HandleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("[Server] Received convert request from %s", r.RemoteAddr)
	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var nodes []*converter.ProxyNode
	var err error

	if req.Input != "" {
		logger.Info("[Server] Processing input (length: %d)", len(req.Input))
		if strings.HasPrefix(req.Input, "http://") || strings.HasPrefix(req.Input, "https://") {
			logger.Info("[Server] Detected URL input, extracting from URL...")
			nodes, err = s.extractor.ExtractFromURL(req.Input)
		} else {
			logger.Info("[Server] Extracting from content...")
			nodes, err = s.extractor.ExtractFromContent(req.Input)
		}
	} else if len(req.URIs) > 0 {
		logger.Info("[Server] Processing %d URIs", len(req.URIs))
		nodes, err = s.extractor.ExtractFromURIs(req.URIs)
	} else {
		s.writeError(w, "input or uris required", http.StatusBadRequest)
		return
	}

	if err != nil {
		logger.Info("[Server] Extraction failed: %v", err)
		s.writeError(w, fmt.Sprintf("extract nodes failed: %v", err), http.StatusBadRequest)
		return
	}

	logger.Info("[Server] Successfully extracted %d nodes, assembling configuration...", len(nodes))
	if req.CustomRequirements != "" {
		logger.Info("[Server] Custom requirements provided: %s", req.CustomRequirements)
	}
	configData, err := s.assembler.AssembleWithRequirements(nodes, req.CustomRequirements)
	if err != nil {
		logger.Info("[Server] Assembly failed: %v", err)
		s.writeError(w, fmt.Sprintf("assemble config failed: %v", err), http.StatusInternalServerError)
		return
	}

	configID := uuid.New().String()
	s.cacheMu.Lock()
	s.cache[configID] = configData
	s.cacheMu.Unlock()

	subURL := s.buildSubscriptionURL(r, configID)
	logger.Info("[Server] Configuration generated successfully, ID: %s", configID)
	logger.Info("[Server] Subscription URL: %s", subURL)

	resp := ConvertResponse{
		ConfigID: configID,
		URL:      subURL,
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}

// HandleConvertStream handles conversion with SSE progress updates
func (s *Server) HandleConvertStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	logger.Info("[Server] Received convert stream request from %s", r.RemoteAddr)
	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"invalid request body\"}\n\n")
		flusher.Flush()
		return
	}

	// Progress callback
	sendProgress := func(step string) {
		data := map[string]string{"step": step}
		jsonData, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", jsonData)
		flusher.Flush()
		logger.Info("[Server] Progress: %s", step)
	}

	var nodes []*converter.ProxyNode
	var err error

	if req.Input != "" {
		logger.Info("[Server] Processing input (length: %d)", len(req.Input))
		if strings.HasPrefix(req.Input, "http://") || strings.HasPrefix(req.Input, "https://") {
			logger.Info("[Server] Detected URL input, extracting from URL...")
			nodes, err = s.extractor.ExtractFromURL(req.Input)
		} else {
			logger.Info("[Server] Extracting from content...")
			nodes, err = s.extractor.ExtractFromContent(req.Input)
		}
	} else if len(req.URIs) > 0 {
		logger.Info("[Server] Processing %d URIs", len(req.URIs))
		nodes, err = s.extractor.ExtractFromURIs(req.URIs)
	} else {
		fmt.Fprintf(w, "event: error\ndata: {\"error\": \"input or uris required\"}\n\n")
		flusher.Flush()
		return
	}

	if err != nil {
		logger.Info("[Server] Extraction failed: %v", err)
		errMsg := map[string]string{"error": fmt.Sprintf("extract nodes failed: %v", err)}
		jsonData, _ := json.Marshal(errMsg)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonData)
		flusher.Flush()
		return
	}

	logger.Info("[Server] Successfully extracted %d nodes, assembling configuration...", len(nodes))
	if req.CustomRequirements != "" {
		logger.Info("[Server] Custom requirements provided: %s", req.CustomRequirements)
	}

	// Send parse step completion
	sendProgress("parse")

	configData, err := s.assembler.AssembleWithProgress(nodes, req.CustomRequirements, sendProgress)
	if err != nil {
		logger.Info("[Server] Assembly failed: %v", err)
		errMsg := map[string]string{"error": fmt.Sprintf("assemble config failed: %v", err)}
		jsonData, _ := json.Marshal(errMsg)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", jsonData)
		flusher.Flush()
		return
	}

	configID := uuid.New().String()
	s.cacheMu.Lock()
	s.cache[configID] = configData
	s.cacheMu.Unlock()

	subURL := s.buildSubscriptionURL(r, configID)
	logger.Info("[Server] Configuration generated successfully, ID: %s", configID)
	logger.Info("[Server] Subscription URL: %s", subURL)

	resp := ConvertResponse{
		ConfigID: configID,
		URL:      subURL,
	}
	jsonData, _ := json.Marshal(resp)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", jsonData)
	flusher.Flush()
}

func (s *Server) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	configID := strings.TrimPrefix(r.URL.Path, "/subscribe/")
	if configID == "" {
		http.NotFound(w, r)
		return
	}

	s.cacheMu.RLock()
	configData, exists := s.cache[configID]
	s.cacheMu.RUnlock()

	if !exists {
		http.NotFound(w, r)
		return
	}

	// Get subscription name from query parameter or use default
	subscriptionName := r.URL.Query().Get("name")
	if subscriptionName == "" {
		subscriptionName = s.subscriptionName
	}

	// Set headers for Clash/Mihomo subscription
	w.Header().Set("Content-Type", contentTypeYAML)
	// Use UTF-8 encoding for subscription name (Clash/Mihomo standard)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s.yaml", url.QueryEscape(subscriptionName)))
	// Optional: Set profile update interval (24 hours)
	w.Header().Set("Profile-Update-Interval", "24")

	w.Write(configData)
}

func (s *Server) HandleNodeToURI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var node converter.ProxyNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		s.writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	uri, err := converter.ProxyToUri(&node)
	if err != nil {
		s.writeError(w, fmt.Sprintf("convert failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{"uri": uri})
}

func (s *Server) HandleURIToNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URI string `json:"uri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	node, err := converter.UriToProxy(req.URI)
	if err != nil {
		s.writeError(w, fmt.Sprintf("parse failed: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	json.NewEncoder(w).Encode(node)
}

func (s *Server) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// HandleExtractNodes extracts proxy nodes from subscription without LLM processing
func (s *Server) HandleExtractNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logger.Info("[Server] Received extract nodes request from %s", r.RemoteAddr)
	var req ExtractNodesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Input == "" {
		s.writeError(w, "input required", http.StatusBadRequest)
		return
	}

	var nodes []*converter.ProxyNode
	var err error

	logger.Info("[Server] Processing input (length: %d)", len(req.Input))
	if strings.HasPrefix(req.Input, "http://") || strings.HasPrefix(req.Input, "https://") {
		logger.Info("[Server] Detected URL input, extracting from URL...")
		nodes, err = s.extractor.ExtractFromURL(req.Input)
	} else {
		logger.Info("[Server] Extracting from content...")
		nodes, err = s.extractor.ExtractFromContent(req.Input)
	}

	if err != nil {
		logger.Info("[Server] Extraction failed: %v", err)
		s.writeError(w, fmt.Sprintf("extract nodes failed: %v", err), http.StatusBadRequest)
		return
	}

	logger.Info("[Server] Successfully extracted %d nodes", len(nodes))

	// Convert nodes to NodeWithURI format
	nodesWithURI := make([]*NodeWithURI, 0, len(nodes))
	for _, node := range nodes {
		uri, err := converter.ProxyToUri(node)
		if err != nil {
			logger.Info("[Server] Failed to convert node %s to URI: %v", node.Name, err)
			uri = "" // Set empty URI if conversion fails
		}
		nodesWithURI = append(nodesWithURI, &NodeWithURI{
			Node: node,
			URI:  uri,
		})
	}

	resp := ExtractNodesResponse{
		Nodes: nodesWithURI,
		Count: len(nodesWithURI),
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	json.NewEncoder(w).Encode(resp)
}
