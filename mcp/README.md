# MCP Package

The `mcp` package provides Model Context Protocol (MCP) client configuration and management for the Product Designer application. It enables integration with external MCP servers like Google Stitch, GitHub MCP, and custom in-memory MCP servers.

## Features

- **Environment-based configuration**: Configure MCP servers via `.env` file
- **Multiple server types**: Support for local (in-memory) and remote (HTTP/SSE) MCP servers
- **Authentication**: OAuth2, API key, and no-authentication modes
- **Tool discovery**: Automatic tool discovery from connected MCP servers
- **ADK integration**: Seamless integration with Google ADK toolsets

## Configuration

Add MCP configuration to your `.env` file:

```bash
# Enable MCP servers (comma-separated list)
MCP_SERVERS=stitch,github

# Default server (optional, defaults to first server)
MCP_DEFAULT_SERVER=stitch

# Google Stitch MCP Configuration
MCP_STITCH_TYPE=remote
MCP_STITCH_ENDPOINT=https://stitch.mcp.google.com
MCP_STITCH_AUTH_TYPE=oauth2
MCP_STITCH_TOKEN_ENV=GOOGLE_API_KEY

# GitHub MCP Configuration
MCP_GITHUB_TYPE=remote
MCP_GITHUB_ENDPOINT=https://api.githubcopilot.com/mcp/
MCP_GITHUB_AUTH_TYPE=oauth2
MCP_GITHUB_TOKEN_ENV=GITHUB_PAT

# Local MCP Server (optional)
MCP_LOCAL_TYPE=local
MCP_LOCAL_TOOLS=get_weather,file_ops
```

### Configuration Options

| Variable | Required | Description |
|----------|----------|-------------|
| `MCP_SERVERS` | No | Comma-separated list of server names to configure |
| `MCP_DEFAULT_SERVER` | No | Default server name for `GetDefaultToolset()` |
| `MCP_<NAME>_TYPE` | Yes | Server type: `local` or `remote` |
| `MCP_<NAME>_ENDPOINT` | Yes (remote) | Remote server URL |
| `MCP_<NAME>_AUTH_TYPE` | No | Auth type: `oauth2`, `api_key`, or `none` (default) |
| `MCP_<NAME>_TOKEN_ENV` | Yes (auth) | Environment variable containing the auth token |
| `MCP_<NAME>_TOOLS` | No (local) | Comma-separated list of tools for local servers |

## Usage

### Basic Integration

```go
import (
    "context"
    "product_designer/pkg/mcp"
    "google.golang.org/adk/agent/llmagent"
)

func setupAgent(ctx context.Context) error {
    // Initialize MCP integration
    mcpIntegration, err := mcp.InitIntegration(ctx)
    if err != nil {
        return fmt.Errorf("failed to init MCP: %w", err)
    }
    defer mcpIntegration.Close()

    // Get toolset for specific MCP server
    stitchTools, err := mcpIntegration.GetToolset("stitch")
    if err != nil {
        return fmt.Errorf("failed to get stitch toolset: %w", err)
    }

    // Create agent with MCP toolset
    agent, err := llmagent.New(llmagent.Config{
        Name:        "design_agent",
        Model:       model,
        Toolsets:    []tool.Toolset{stitchTools},
    })

    return nil
}
```

### Using in Multi-Agent System (main.go)

The `cmd/server/main.go` demonstrates a complete multi-agent system with MCP integration:

```go
func main() {
    ctx := context.Background()

    // Initialize model
    modelFactory, _ := infrastructure.InitProviders(ctx)
    model, _ := modelFactory.GetModelByProfile("balanced")

    // Initialize MCP integration
    mcpIntegration, _ := mcp.InitIntegration(ctx)
    defer mcpIntegration.Close()

    // Create frontend orchestrator with MCP Stitch tools
    frontendOrchestrator, _ := createFrontendOrchestrator(model, mcpIntegration)

    // Create other agents...

    // Build master pipeline with all agents
    masterPipeline, _ := sequentialagent.New(sequentialagent.Config{
        AgentConfig: agent.Config{
            Name:        "ProductDesignerMaster",
            SubAgents:   []agent.Agent{autoCoT, clarifier, projectManager, builderPipeline},
        },
    })
}

func createFrontendOrchestrator(m model.LLM, mcpIntegration *mcp.Integration) (agent.Agent, error) {
    if mcpIntegration != nil {
        stitchTools, err := mcpIntegration.GetToolset("stitch")
        if err == nil {
            return orchestrators.NewFrontendOrchestratorWithMCP(m, stitchTools)
        }
    }
    return orchestrators.NewFrontendOrchestrator(m)
}
```

### Using Multiple MCP Servers

```go
// Get all available MCP toolsets
allToolsets, err := mcpIntegration.GetAllToolsets()
if err != nil {
    return err
}

// Create agent with all MCP tools
agent, err := llmagent.New(llmagent.Config{
    Name:        "multi_mcp_agent",
    Model:       model,
    Toolsets:    allToolsets,
})
```

### Using Default Server

```go
// Get default MCP toolset
defaultTools, err := mcpIntegration.GetDefaultToolset()
if err != nil {
    return err
}
```

### Listing Available Tools

```go
// List all tools across all connected MCP servers
tools, err := mcpIntegration.Registry().ListAvailableTools(ctx)
if err != nil {
    return err
}

for serverName, toolNames := range tools {
    fmt.Printf("Server %s provides: %v\n", serverName, toolNames)
}
```

## Architecture

```
pkg/mcp/
├── config.go           # Configuration parsing and validation
├── client.go           # MCP client connection management
├── registry.go         # Server registry and toolset provider
├── integration.go      # High-level integration helpers
└── util/
    ├── context_manager.go    # Context management tools
    ├── prompt_injector.go    # Prompt injection utilities
    ├── tool_discovery.go     # Tool discovery and caching
    └── tool_formatter.go     # Tool documentation formatting
```

### Connection Flow

1. **LoadConfig()**: Parses environment variables into `Config` struct
2. **InitMCPRegistry()**: Creates clients for all configured servers
3. **Client.Connect()**: Establishes MCP session and retrieves server info
4. **Registry.GetToolset()**: Creates ADK toolset from MCP connection

### Server Types

**Remote Servers**: Connect to external MCP servers via HTTP/SSE with authentication
- OAuth2 token-based auth (e.g., Google Stitch, GitHub MCP)
- API key auth
- No authentication

**Local Servers**: In-memory MCP servers for custom tools
- Pre-registered tool handlers
- No network latency
- Useful for testing and custom integrations

## Error Handling

The MCP package provides comprehensive error handling:

- Configuration validation at load time
- Connection failures are logged but don't block other servers
- Tool discovery failures are non-fatal
- Graceful shutdown on `Close()`

## Example .env Configuration

```bash
# LLM Providers
GOOGLE_API_KEY=your_google_api_key
DEEPSEEK_API_KEY=your_deepseek_key

# MCP Servers
MCP_SERVERS=stitch
MCP_DEFAULT_SERVER=stitch

# Google Stitch MCP
MCP_STITCH_TYPE=remote
MCP_STITCH_ENDPOINT=https://stitch.mcp.google.com
MCP_STITCH_AUTH_TYPE=oauth2
MCP_STITCH_TOKEN_ENV=GOOGLE_API_KEY
```

## Testing

```go
func TestMCPConfig(t *testing.T) {
    os.Setenv("MCP_SERVERS", "test")
    os.Setenv("MCP_TEST_TYPE", "local")
    os.Setenv("MCP_TEST_TOOLS", "tool1,tool2")
    
    config, err := mcp.LoadConfig()
    if err != nil {
        t.Fatal(err)
    }
    
    if len(config.Servers) != 1 {
        t.Errorf("expected 1 server, got %d", len(config.Servers))
    }
}
```

## See Also

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [Google ADK Documentation](https://pkg.go.dev/google.golang.org/adk)
- [MCP Go SDK](https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk)
