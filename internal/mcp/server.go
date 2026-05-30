package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/steevin/neuron-cli/internal/config"
	"github.com/steevin/neuron-cli/internal/notes"
	"github.com/steevin/neuron-cli/internal/search"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NeuronMCPServer wraps the MCP server and provides handlers that interact with the vault.
type NeuronMCPServer struct {
	store  *notes.Store
	index  *search.Index
	config *config.Config
	server *server.MCPServer
}

// NewServer creates a new MCP server.
func NewServer(cfg *config.Config, store *notes.Store, index *search.Index) (*NeuronMCPServer, error) {
	mcpServer := server.NewMCPServer("neuron", "0.1.0", server.WithResourceCapabilities(true, true), server.WithPromptCapabilities(true))
	
	s := &NeuronMCPServer{
		store:  store,
		index:  index,
		config: cfg,
		server: mcpServer,
	}

	s.registerTools()
	s.registerResources()
	return s, nil
}

// Start runs the MCP stdio server, blocking until it shuts down.
func (s *NeuronMCPServer) Start() error {
	stdioServer := server.NewStdioServer(s.server)
	return stdioServer.Listen(context.Background(), os.Stdin, os.Stdout)
}

func (s *NeuronMCPServer) registerTools() {
	// 1. search_notes
	searchNotesTool := mcp.NewTool("search_notes",
		mcp.WithDescription("Full-text search across the knowledge vault"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query terms"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 10)"),
		),
	)
	s.server.AddTool(searchNotesTool, s.handleSearchNotes)

	// 2. get_note
	getNoteTool := mcp.NewTool("get_note",
		mcp.WithDescription("Retrieve a full note by its ID or exact title"),
		mcp.WithString("id_or_title",
			mcp.Required(),
			mcp.Description("Note ID or title"),
		),
	)
	s.server.AddTool(getNoteTool, s.handleGetNote)

	// 3. create_note
	createNoteTool := mcp.NewTool("create_note",
		mcp.WithDescription("Create a new note in the vault"),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("Title of the note"),
		),
		mcp.WithString("content",
			mcp.Description("Markdown body content"),
		),
		mcp.WithString("tags",
			mcp.Description("Comma-separated list of tags (e.g. 'ideas, project')"),
		),
	)
	s.server.AddTool(createNoteTool, s.handleCreateNote)

	// 4. update_note
	updateNoteTool := mcp.NewTool("update_note",
		mcp.WithDescription("Update an existing note's content"),
		mcp.WithString("id_or_title",
			mcp.Required(),
			mcp.Description("Note ID or title"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("New complete markdown body content (replaces existing content)"),
		),
	)
	s.server.AddTool(updateNoteTool, s.handleUpdateNote)

	// 5. list_notes
	listNotesTool := mcp.NewTool("list_notes",
		mcp.WithDescription("List notes, optionally filtered by tag"),
		mcp.WithString("tag",
			mcp.Description("Filter by tag"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default: 20)"),
		),
	)
	s.server.AddTool(listNotesTool, s.handleListNotes)

	// 6. get_daily
	getDailyTool := mcp.NewTool("get_daily",
		mcp.WithDescription("Get or create today's daily note"),
	)
	s.server.AddTool(getDailyTool, s.handleGetDaily)
}

func (s *NeuronMCPServer) handleSearchNotes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("missing required argument 'query'"), nil
	}

	limit := req.GetInt("limit", 10)

	results := s.index.Search(query, limit)
	
	type searchRes struct {
		ID      string   `json:"id"`
		Title   string   `json:"title"`
		Tags    []string `json:"tags"`
		Updated string   `json:"updated"`
		Excerpt string   `json:"excerpt"`
	}

	var out []searchRes
	for _, r := range results {
		excerpt := r.Note.Content
		if len(excerpt) > 150 {
			excerpt = excerpt[:150] + "..."
		}
		out = append(out, searchRes{
			ID:      r.Note.ID,
			Title:   r.Note.Title,
			Tags:    r.Note.Tags,
			Updated: r.Note.Updated.Format(time.RFC3339),
			Excerpt: excerpt,
		})
	}

	b, _ := json.Marshal(out)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *NeuronMCPServer) handleGetNote(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idOrTitle, err := req.RequireString("id_or_title")
	if err != nil {
		return mcp.NewToolResultError("missing 'id_or_title'"), nil
	}

	note, err := s.store.Get(idOrTitle)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("note not found: %v", err)), nil
	}

	b, _ := json.Marshal(note)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *NeuronMCPServer) handleCreateNote(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError("missing 'title'"), nil
	}
	content := req.GetString("content", "")
	tagsStr := req.GetString("tags", "")
	
	var tags []string
	if tagsStr != "" {
		for _, t := range strings.Split(tagsStr, ",") {
			tags = append(tags, strings.TrimSpace(t))
		}
	}

	note, err := s.store.Create(title, tags, content)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create note: %v", err)), nil
	}

	// Rebuild index for new note
	s.index.IndexNote(note)

	type createRes struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Path  string `json:"path"`
	}
	res := createRes{ID: note.ID, Title: note.Title, Path: note.Path}
	b, _ := json.Marshal(res)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *NeuronMCPServer) handleUpdateNote(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idOrTitle, err := req.RequireString("id_or_title")
	if err != nil {
		return mcp.NewToolResultError("missing 'id_or_title'"), nil
	}
	content, err := req.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError("missing 'content'"), nil
	}

	note, err := s.store.Get(idOrTitle)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("note not found: %v", err)), nil
	}

	note.Content = content
	if err := s.store.Update(note); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update note: %v", err)), nil
	}

	s.index.IndexNote(note)

	type updateRes struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Updated string `json:"updated"`
	}
	res := updateRes{ID: note.ID, Title: note.Title, Updated: note.Updated.Format(time.RFC3339)}
	b, _ := json.Marshal(res)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *NeuronMCPServer) handleListNotes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	opts := notes.ListOptions{Limit: 20}
	limit := req.GetInt("limit", 20)
	opts.Limit = limit
	
	tag := req.GetString("tag", "")
	if tag != "" {
		opts.Tags = []string{tag}
	}

	nList, err := s.store.List(opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	type listRes struct {
		ID      string   `json:"id"`
		Title   string   `json:"title"`
		Tags    []string `json:"tags"`
		Updated string   `json:"updated"`
	}
	var out []listRes
	for _, n := range nList {
		out = append(out, listRes{
			ID:      n.ID,
			Title:   n.Title,
			Tags:    n.Tags,
			Updated: n.Updated.Format(time.RFC3339),
		})
	}

	b, _ := json.Marshal(out)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *NeuronMCPServer) handleGetDaily(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title := "Daily " + time.Now().Format("2006-01-02")
	note, err := s.store.Get(title)
	if err != nil {
		// Create it
		content := "## 🎯 Today's goals\n- [ ] \n\n## 📝 Notes\n\n## 🔗 Links\n"
		note, err = s.store.Create(title, []string{"daily"}, content)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create daily: %v", err)), nil
		}
		s.index.IndexNote(note)
	}
	
	b, _ := json.Marshal(note)
	return mcp.NewToolResultText(string(b)), nil
}
