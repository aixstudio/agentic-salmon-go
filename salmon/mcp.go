package salmon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "0.1.0"
const ServerName = "agentic-salmon-go-knowledge"

type SearchArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}
type SearchOutput struct {
	Query  string  `json:"query"`
	Model  string  `json:"model"`
	Chunks []Chunk `json:"chunks"`
}
type SourceArgs struct {
	SourceID string `json:"source_id"`
}

func NewMCPServer(r *Retriever) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: ServerName, Version: Version}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "search_knowledge", Description: "Read-only ranked retrieval over reviewed Agentic Salmon sources."}, func(ctx context.Context, req *mcp.CallToolRequest, args SearchArgs) (*mcp.CallToolResult, SearchOutput, error) {
		chunks, err := r.Search(args.Query, args.TopK)
		return nil, SearchOutput{args.Query, RetrievalModel, chunks}, err
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_source", Description: "Resolve a reviewed source identifier and its content digest."}, func(ctx context.Context, req *mcp.CallToolRequest, args SourceArgs) (*mcp.CallToolResult, Source, error) {
		source, err := r.GetSource(args.SourceID)
		return nil, source, err
	})
	return s
}
func ServeKnowledge(ctx context.Context) error {
	r, err := NewRetriever()
	if err != nil {
		return err
	}
	return NewMCPServer(r).Run(ctx, &mcp.StdioTransport{})
}

// MCPKnowledgePort launches only the configured executable with the read-only
// knowledge-server subcommand. It never accepts a shell command from evidence.
type MCPKnowledgePort struct{ Executable string }

func (p MCPKnowledgePort) Search(ctx context.Context, query string, k int) (KnowledgeResult, error) {
	var out KnowledgeResult
	if p.Executable == "" {
		return out, fmt.Errorf("MCP executable must be configured")
	}
	if k < 1 || k > 5 {
		return out, fmt.Errorf("invalid top_k")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Executable, "mcp-server")
	cmd.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "agentic-salmon-go", Version: Version}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return out, fmt.Errorf("MCP initialize: %w", err)
	}
	defer session.Close()
	start := time.Now()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "search_knowledge", Arguments: SearchArgs{query, k}})
	if err != nil {
		return out, fmt.Errorf("MCP search_knowledge: %w", err)
	}
	if result.IsError {
		return out, fmt.Errorf("MCP search_knowledge returned a tool error")
	}
	if result.StructuredContent == nil {
		return out, fmt.Errorf("MCP search_knowledge omitted structured content")
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return out, err
	}
	var payload SearchOutput
	if err = decodeStrict(data, &payload); err != nil {
		return out, err
	}
	if payload.Query != query || len(payload.Chunks) > k || payload.Model == "" {
		return out, fmt.Errorf("MCP result violates query contract")
	}
	for _, c := range payload.Chunks {
		if c.SHA256 != digest([]byte(c.Text)) || c.ID == "" || c.URL == "" {
			return out, fmt.Errorf("MCP returned invalid citation")
		}
	}
	init := session.InitializeResult()
	out = KnowledgeResult{Query: payload.Query, Model: payload.Model, Chunks: payload.Chunks, MCPCall: MCPCall{ServerName: init.ServerInfo.Name, ServerVersion: init.ServerInfo.Version, ProtocolVersion: init.ProtocolVersion, ToolName: "search_knowledge", DurationMS: float64(time.Since(start).Microseconds()) / 1000, Success: true}}
	return out, nil
}
