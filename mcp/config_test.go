// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"context"
	"os"
	"testing"
)

func TestLoadConfig_NoServers(t *testing.T) {
	// Clear MCP_SERVERS env var
	os.Unsetenv("MCP_SERVERS")
	
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	
	if len(config.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(config.Servers))
	}
}

func TestLoadConfig_RemoteServer(t *testing.T) {
	os.Setenv("MCP_SERVERS", "stitch")
	os.Setenv("MCP_STITCH_TYPE", "remote")
	os.Setenv("MCP_STITCH_ENDPOINT", "https://stitch.example.com")
	os.Setenv("MCP_STITCH_AUTH_TYPE", "oauth2")
	os.Setenv("MCP_STITCH_TOKEN_ENV", "GOOGLE_API_KEY")
	os.Setenv("GOOGLE_API_KEY", "test_key")
	defer func() {
		os.Unsetenv("MCP_SERVERS")
		os.Unsetenv("MCP_STITCH_TYPE")
		os.Unsetenv("MCP_STITCH_ENDPOINT")
		os.Unsetenv("MCP_STITCH_AUTH_TYPE")
		os.Unsetenv("MCP_STITCH_TOKEN_ENV")
		os.Unsetenv("GOOGLE_API_KEY")
	}()
	
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	
	if len(config.Servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(config.Servers))
	}
	
	stitchConfig, ok := config.Servers["stitch"]
	if !ok {
		t.Fatal("stitch server not found in config")
	}
	
	if stitchConfig.Type != ServerTypeRemote {
		t.Errorf("expected type %v, got %v", ServerTypeRemote, stitchConfig.Type)
	}
	
	if stitchConfig.Endpoint != "https://stitch.example.com" {
		t.Errorf("expected endpoint https://stitch.example.com, got %s", stitchConfig.Endpoint)
	}
	
	if stitchConfig.AuthType != AuthTypeOAuth2 {
		t.Errorf("expected auth type %v, got %v", AuthTypeOAuth2, stitchConfig.AuthType)
	}
}

func TestLoadConfig_LocalServer(t *testing.T) {
	os.Setenv("MCP_SERVERS", "local")
	os.Setenv("MCP_LOCAL_TYPE", "local")
	os.Setenv("MCP_LOCAL_TOOLS", "get_weather,file_ops")
	defer func() {
		os.Unsetenv("MCP_SERVERS")
		os.Unsetenv("MCP_LOCAL_TYPE")
		os.Unsetenv("MCP_LOCAL_TOOLS")
	}()
	
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	
	localConfig, ok := config.Servers["local"]
	if !ok {
		t.Fatal("local server not found in config")
	}
	
	if localConfig.Type != ServerTypeLocal {
		t.Errorf("expected type %v, got %v", ServerTypeLocal, localConfig.Type)
	}
	
	if len(localConfig.Tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(localConfig.Tools))
	}
	
	if localConfig.Tools[0] != "get_weather" {
		t.Errorf("expected first tool get_weather, got %s", localConfig.Tools[0])
	}
}

func TestLoadConfig_MissingRequiredFields(t *testing.T) {
	os.Setenv("MCP_SERVERS", "invalid")
	os.Setenv("MCP_INVALID_TYPE", "remote")
	// Missing ENDPOINT for remote server
	defer os.Unsetenv("MCP_SERVERS")
	
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing endpoint, got nil")
	}
}

func TestLoadConfig_InvalidServerType(t *testing.T) {
	os.Setenv("MCP_SERVERS", "invalid")
	os.Setenv("MCP_INVALID_TYPE", "invalid_type")
	defer func() {
		os.Unsetenv("MCP_SERVERS")
		os.Unsetenv("MCP_INVALID_TYPE")
	}()
	
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid server type, got nil")
	}
}

func TestLoadConfig_InvalidAuthType(t *testing.T) {
	os.Setenv("MCP_SERVERS", "invalid")
	os.Setenv("MCP_INVALID_TYPE", "remote")
	os.Setenv("MCP_INVALID_ENDPOINT", "https://example.com")
	os.Setenv("MCP_INVALID_AUTH_TYPE", "invalid_auth")
	defer func() {
		os.Unsetenv("MCP_SERVERS")
		os.Unsetenv("MCP_INVALID_TYPE")
		os.Unsetenv("MCP_INVALID_ENDPOINT")
		os.Unsetenv("MCP_INVALID_AUTH_TYPE")
	}()
	
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid auth type, got nil")
	}
}

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
	}{
		{
			name: "valid remote-config",
			config: &ServerConfig{
				Name:      "test",
				Type:      ServerTypeRemote,
				Endpoint:  "https://example.com",
				AuthType:  AuthTypeOAuth2,
				TokenEnv:  "TEST_TOKEN",
				Enabled:   true,
			},
			wantErr: false,
		},
		{
			name: "valid-local-config",
			config: &ServerConfig{
				Name:     "local",
				Type:     ServerTypeLocal,
				AuthType: AuthTypeNone,
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "missing-name",
			config: &ServerConfig{
				Type: ServerTypeLocal,
			},
			wantErr: true,
		},
		{
			name: "remote-missing-endpoint",
			config: &ServerConfig{
				Name:   "test",
				Type:   ServerTypeRemote,
				Enabled: true,
			},
			wantErr: true,
		},
		{
			name: "oauth-missing-token-env",
			config: &ServerConfig{
				Name:     "test",
				Type:     ServerTypeRemote,
				Endpoint: "https://example.com",
				AuthType: AuthTypeOAuth2,
				Enabled:  true,
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServerConfig_GetAuthToken(t *testing.T) {
	os.Setenv("TEST_TOKEN", "secret_token_123")
	defer os.Unsetenv("TEST_TOKEN")
	
	config := &ServerConfig{
		TokenEnv: "TEST_TOKEN",
	}
	
	token := config.GetAuthToken()
	if token != "secret_token_123" {
		t.Errorf("expected token secret_token_123, got %s", token)
	}
}

func TestInitMCPRegistry(t *testing.T) {
	// Test with no servers configured
	os.Unsetenv("MCP_SERVERS")
	
	ctx := context.Background()
	registry, err := InitMCPRegistry(ctx)
	if err != nil {
		t.Fatalf("InitMCPRegistry() error = %v", err)
	}
	
	if len(registry.clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(registry.clients))
	}
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "single-value",
			input:  "stitch",
			expect: []string{"stitch"},
		},
		{
			name:   "multiple-values",
			input:  "stitch,github,local",
			expect: []string{"stitch", "github", "local"},
		},
		{
			name:   "with-spaces",
			input:  "stitch, github , local",
			expect: []string{"stitch", "github", "local"},
		},
		{
			name:   "empty-string",
			input:  "",
			expect: []string{},
		},
		{
			name:   "only-commas",
			input:  ",,",
			expect: []string{},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommaSeparated(tt.input)
			if len(result) != len(tt.expect) {
				t.Errorf("expected %d items, got %d", len(tt.expect), len(result))
				return
			}
			for i, v := range result {
				if v != tt.expect[i] {
					t.Errorf("item %d: expected %s, got %s", i, tt.expect[i], v)
				}
			}
		})
	}
}
