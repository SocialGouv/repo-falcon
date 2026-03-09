package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "falcon"
	serverVersion   = "0.1.0"
)

// JSONRPCRequest is a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server is a minimal MCP server over stdio.
type Server struct {
	graph       *GraphIndex
	tools       []ToolDef
	handler     func(name string, args map[string]any, s *Server) (string, error)
	RepoRoot    string // repo root for re-indexing
	SnapshotDir string // snapshot dir for re-indexing and reloading
}

// NewServer creates a new MCP server backed by the given graph index.
func NewServer(g *GraphIndex, repoRoot, snapshotDir string) *Server {
	return &Server{
		graph:       g,
		tools:       AllTools(),
		handler:     HandleToolCall,
		RepoRoot:    repoRoot,
		SnapshotDir: snapshotDir,
	}
}

// ReloadGraph reloads the graph from the snapshot directory.
func (s *Server) ReloadGraph(g *GraphIndex) {
	s.graph = g
}

// Serve reads JSON-RPC requests from r and writes responses to w.
// It runs until r is closed or an error occurs.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	// MCP uses newline-delimited JSON.
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			resp := JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "parse error"},
			}
			if werr := writeResponse(w, resp); werr != nil {
				return werr
			}
			continue
		}

		resp := s.handleRequest(req)
		if resp != nil {
			if err := writeResponse(w, *resp); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}

func (s *Server) handleRequest(req JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    serverName,
					"version": serverVersion,
				},
			},
		}

	case "notifications/initialized":
		// Notification, no response.
		return nil

	case "tools/list":
		defs := make([]map[string]any, len(s.tools))
		for i, t := range s.tools {
			defs[i] = map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			}
		}
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": defs,
			},
		}

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &RPCError{Code: -32602, Message: "invalid params"},
			}
		}

		text, err := s.handler(params.Name, params.Arguments, s)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
					},
					"isError": true,
				},
			}
		}

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": text},
				},
			},
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
		}
	}
}

func writeResponse(w io.Writer, resp JSONRPCResponse) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
