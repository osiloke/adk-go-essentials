# ADK Essentials

A reusable Go module providing essential utilities for building applications with [Google ADK (Agent Development Kit)](https://pkg.go.dev/google.golang.org/adk).

This module extracts common infrastructure, observability, and MCP (Model Context Protocol) utilities from a production product designer application, making them available for reuse across multiple projects.

## Features

### Infrastructure Package (`infrastructure/`)

Provides model implementations and management utilities for various LLM providers:

- **DeepSeek Model**: Native integration with DeepSeek's API using the `cohesion-org/deepseek-go` client
- **OpenAI-compatible Model**: OpenAI API wrapper that also works with OpenAI-compatible endpoints (DeepSeek OpenAI, OpenRouter, custom endpoints)
- **Retry Model**: Resilient wrapper with exponential backoff, jitter, and smart retry logic
- **Model Factory**: Dynamic model selection based on environment variables and capability profiles (quick, fast, smart, reasoning, balanced)

**Supported Providers**:
- Google Gemini (via ADK's built-in Gemini model)
- DeepSeek (native API)
- DeepSeek (OpenAI-compatible API)
- OpenAI (GPT-4o, GPT-4o-mini)
- OpenRouter (Claude, Llama, etc.)

### Observability Package (`observability/`)

Logging and artifact management utilities:

- **Logger Interface**: Structured logging interface with `slog` backend
- **ADK Callbacks**: Pre-built callbacks for logging agent lifecycle, LLM requests/responses
- **Artifact Management**: Save agent outputs as artifacts with automatic MIME type detection

### MCP Package (`mcp/`)

Complete Model Context Protocol client implementation:

- **Configuration**: Environment-based configuration for multiple MCP servers
- **Server Types**: Support for local (in-memory) and remote (HTTP/SSE) servers
- **Authentication**: OAuth2, API key, and no-authentication modes
- **Tool Discovery**: Automatic tool discovery and caching
- **ADK Integration**: Seamless integration with Google ADK toolsets
- **Utility Tools**:
  - Context management (set/get/list/clear context)
  - Prompt injection utilities
  - Tool formatting (markdown, tables, lists)
- **Stitch Integration**: Specialized tools for Google Stitch MCP (design-to-code)

## Installation

```bash
go get github.com/osiloke/adk-go-essentials
```

## Usage

### Model Factory

```go
import (
    "context"
    "github.com/osiloke/adk-go-essentials/infrastructure"
)

func main() {
    ctx := context.Background()
    
    // Initialize providers from environment variables
    factory, err := infrastructure.InitProviders(ctx)
    if err != nil {
        panic(err)
    }
    
    // Get model by profile
    model, err := factory.GetModelByProfile("smart")
    if err != nil {
        panic(err)
    }
    
    // Or get model by ID
    model, err = factory.GetModelByID("gemini/gemini-2.0-flash")
    if err != nil {
        panic(err)
    }
}
```

**Required Environment Variables**:
```bash
# At least one of these must be set:
GOOGLE_API_KEY=your_key        # For Gemini models
DEEPSEEK_API_KEY=your_key      # For DeepSeek native
DEEPSEEK_OPENAI_KEY=your_key   # For DeepSeek via OpenAI API
OPENAI_API_KEY=your_key        # For OpenAI
OPENROUTER_API_KEY=your_key    # For OpenRouter models
```

### Retry Model Wrapper

```go
import (
    "time"
    "github.com/osiloke/adk-go-essentials/infrastructure"
)

// Wrap any model with retry logic
model = infrastructure.NewRetryModel(
    innerModel,
    3,              // max retries
    2*time.Second,  // base delay
    30*time.Second, // max delay
)
```

### Observability

```go
import (
    "github.com/osiloke/adk-go-essentials/observability"
    "google.golang.org/adk/agent/llmagent"
)

// Use the global logger
observability.Log.Infof("Starting agent...")

// Add logging callbacks to your LLM agent
agent, _ := llmagent.New(llmagent.Config{
    Name:  "my-agent",
    Model: model,
    BeforeModelCallbacks: []llmagent.BeforeModelCallback{
        observability.LogLLMRequest,
    },
    AfterModelCallbacks: []llmagent.AfterModelCallback{
        func(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
            return observability.LogLLMResponse(ctx, resp, err)
        },
    },
})
```

### Save Artifacts

```go
import (
    "github.com/osiloke/adk-go-essentials/observability"
    "google.golang.org/adk/agent"
)

// Add as AfterAgentCallback to save outputs
config := agent.Config{
    Name: "my-agent",
    AfterAgentCallbacks: []agent.AfterAgentCallback{
        observability.SaveOutputAsArtifact("result.md", "output_key"),
    },
}
```

### MCP Integration

```go
import (
    "context"
    "github.com/osiloke/adk-go-essentials/mcp"
)

func setupMCP(ctx context.Context) error {
    // Initialize from environment
    integration, err := mcp.InitIntegration(ctx)
    if err != nil {
        return err
    }
    defer integration.Close()
    
    // Get toolset for a specific server
    stitchTools, err := integration.GetToolset("stitch")
    if err != nil {
        return err
    }
    
    // Or get all toolsets
    allTools, err := integration.GetAllToolsets()
    if err != nil {
        return err
    }
    
    // Use with ADK agents
    // agent, _ := llmagent.New(llmagent.Config{
    //     Toolsets: []tool.Toolset{stitchTools},
    // })
    
    return nil
}
```

**MCP Configuration Example** (`.env`):
```bash
# Enable MCP servers
MCP_SERVERS=stitch,github

# Google Stitch configuration
MCP_STITCH_TYPE=remote
MCP_STITCH_ENDPOINT=https://stitch.mcp.google.com
MCP_STITCH_AUTH_TYPE=oauth2
MCP_STITCH_TOKEN_ENV=GOOGLE_API_KEY

# GitHub MCP configuration
MCP_GITHUB_TYPE=remote
MCP_GITHUB_ENDPOINT=https://api.githubcopilot.com/mcp/
MCP_GITHUB_AUTH_TYPE=oauth2
MCP_GITHUB_TOKEN_ENV=GITHUB_PAT
```

### MCP Utilities

#### Tool Discovery and Formatting

```go
import (
    "context"
    "time"
    "github.com/osiloke/adk-go-essentials/mcp/util"
)

// Create discovery service with caching
discovery := util.NewToolDiscoveryService(15 * time.Minute)

// Discover tools from an MCP client
tools, err := discovery.DiscoverTools(ctx, client, "stitch")
if err != nil {
    return err
}

// Format as markdown for prompt injection
markdown := util.FormatToolsAsMarkdown(tools, nil)

// Or use other formats
table := util.FormatToolsAsTable(tools)
list := util.FormatToolsAsList(tools)
```

#### Context Management

```go
import (
    "github.com/osiloke/adk-go-essentials/mcp/util"
)

// Create context manager
manager := util.NewContextManager()

// Create tools for context management
setTool, _ := util.NewSetContextTool(manager)
getTool, _ := util.NewGetContextTool(manager)
listTool, _ := util.NewListContextTool(manager)
clearTool, _ := util.NewClearContextTool(manager)

// Add these tools to your agent
```

#### Prompt Injection

```go
import (
    "github.com/osiloke/adk-go-essentials/mcp/util"
)

// Create prompt injector
injector := util.NewPromptInjector(discoveryService, contextManager)

// Create callback to inject tool documentation
callback := injector.InjectToolsCallback("stitch", nil)

// Add to agent's BeforeModelCallbacks
// This will inject markdown documentation for all tools from the server
```

### State Package (`state/`)

Session state management, validation, and injection utilities:

- **State Injection**: `InjectState`, `InjectSpecificKeys` — inject session state into LLM prompts
- **Key Validation**: `CheckRequiredKeysExist` — verify required state before agent execution
- **Artifact Loading**: `PreCheckAndLoadArtifact`, `PreCheckAndLoadArtifacts` — load artifacts into state before loops
- **Artifact Tool**: `LoadArtifactToStateTool` — agents can call this to load artifacts at runtime
- **PreCheck Agent**: `NewPreCheckAgent` — standalone agent that can escalate to stop loops
- **AutoCoT Parser**: `ParseAutoCoTResult`, `MapAutoCoTResult` — robust JSON parsing with graceful fallbacks

```go
import (
    "github.com/osiloke/adk-go-essentials/state"
    "google.golang.org/adk/agent/llmagent"
)

// Inject specific state keys into every LLM prompt
agent, _ := llmagent.New(llmagent.Config{
    Name:  "my-agent",
    Model: model,
    BeforeModelCallbacks: []llmagent.BeforeModelCallback{
        state.InjectSpecificKeys("web_reqs", "execution_plan"),
    },
})

// Validate required keys before agent runs
config := agent.Config{
    BeforeAgentCallbacks: []agent.BeforeAgentCallback{
        state.CheckRequiredKeysExist([]string{"execution_plan", "output_key"}),
    },
}
```

### Agent Package (`agent/`)

Agent utilities for robust loop execution:

- **Safe Wrapper**: `Safe(agent)` — prevents ADK loop panics on nil events (workaround for ADK bug)

```go
import "github.com/osiloke/adk-go-essentials/agent"

// Wrap agents to prevent loop panics on errors
safeAgent := agent.Safe(myAgent)
```

### Tools Package (`tools/`)

Reusable agent tools:

- **Exit Loop Tool**: `NewExitLoopTool()` — allows agents to signal loop termination via `Escalate = true`

```go
import "github.com/osiloke/adk-go-essentials/tools"

// Create exit loop tool
exitTool, _ := tools.NewExitLoopTool()

// Add to agent's tools so it can terminate loops
```

### Skills Package (`skills/`)

Skill-based agent capability system with frontmatter parsing:

- **Registry**: Load skills from directories (SKILL.md files with YAML frontmatter)
- **Skill Tools**: `list_skills`, `activate_skill`, `load_reference`, `run_skill_script`
- **Prompt Injection**: `InjectActiveSkills`, `InjectActiveSkillsWithFilter` — inject activated skills into prompts

```go
import (
    "github.com/osiloke/adk-go-essentials/skills"
)

// Load skills from directory
registry := skills.NewRegistry()
registry.LoadFromDir("./.product_designer/use_case_skills")

// Create skill tools for agents
listTool, _ := skills.NewListSkillsTool(registry)
activateTool, _ := skills.NewActivateSkillTool(registry)
refTool, _ := skills.NewListReferencesTool(registry)
loadRefTool, _ := skills.NewLoadReferenceTool(registry)

// Inject active skills into prompts
agent, _ := llmagent.New(llmagent.Config{
    BeforeModelCallbacks: []llmagent.BeforeModelCallback{
        skills.InjectActiveSkills,
        // Or filter by directory:
        skills.InjectActiveSkillsWithFilter("use_case_skills"),
    },
})
```

## Architecture

```
adk-go-essentials/
├── infrastructure/          # LLM provider implementations
│   ├── model_factory.go     # Dynamic model selection
│   ├── retry_model.go       # Retry wrapper with backoff
│   ├── deepseek_model.go    # DeepSeek native client
│   └── openai_model.go      # OpenAI-compatible client
├── observability/           # Logging and artifacts
│   ├── logger.go            # Logger interface and ADK callbacks
│   └── artifacts.go         # Artifact saving utilities
├── mcp/                     # Model Context Protocol
│   ├── config.go            # Configuration parsing
│   ├── client.go            # MCP client management
│   ├── registry.go          # Server registry
│   ├── integration.go       # ADK integration helpers
│   ├── resilient_http_client.go  # HTTP client with retries/circuit breaker
│   ├── util/                # MCP utilities
│   │   ├── context_manager.go    # Context management tools
│   │   ├── prompt_injector.go    # Prompt injection utilities
│   │   ├── tool_discovery.go     # Tool discovery and caching
│   │   └── tool_formatter.go     # Tool documentation formatting
├── state/                   # Session state utilities
│   ├── injector.go          # State injection callbacks
│   └── validator.go         # Key/artifact validation, pre-check callbacks
├── agent/                   # Agent utilities
│   └── safe.go              # Safe wrapper to prevent loop panics
├── tools/                   # Reusable agent tools
│   └── exit_loop.go         # Exit loop tool
├── skills/                  # Skill-based capability system
│   ├── registry.go          # Skill loading and frontmatter parsing
│   ├── callback.go          # Active skill prompt injection
│   └── tools.go             # Skill tools (list, activate, references, scripts)
├── go.mod
└── README.md
```

## Dependencies

This module depends on:

- `google.golang.org/adk` - Google ADK core
- `google.golang.org/genai` - Google AI SDK
- `github.com/modelcontextprotocol/go-sdk` - MCP SDK
- `github.com/cohesion-org/deepseek-go` - DeepSeek client
- `github.com/sashabaranov/go-openai` - OpenAI client
- `golang.org/x/oauth2` - OAuth2 support

## License

Apache 2.0 (same as source project)

## Contributing

This module is extracted from a production application. Issues and improvements welcome.
