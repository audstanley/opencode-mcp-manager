package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetConfigPath(t *testing.T) {
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", "/custom/path")
	defer os.Setenv("HOME", originalHome)

	path := getConfigPath()
	expected := "/custom/path/.config/opencode/opencode.json"

	if path != expected {
		t.Errorf("getConfigPath() = %q, want %q", path, expected)
	}
}

func TestParseMCP_ValidConfig(t *testing.T) {
	data := []byte(`{
		"mcp": {
			"server1": {
				"type": "local",
				"command": "test",
				"enabled": true
			},
			"server2": {
				"type": "remote",
				"url": "http://example.com",
				"enabled": false
			}
		}
	}`)

	_, mcpList := parseMCP(data)

	if len(mcpList) != 2 {
		t.Errorf("parseMCP() list length = %d, want 2", len(mcpList))
	}

	server1 := mcpList[0]
	if server1.Name != "server1" {
		t.Errorf("server1.Name = %q, want %q", server1.Name, "server1")
	}
	if server1.Type != "local" {
		t.Errorf("server1.Type = %q, want %q", server1.Type, "local")
	}
	if !server1.Enabled {
		t.Error("server1.Enabled = false, want true")
	}

	server2 := mcpList[1]
	if server2.Name != "server2" {
		t.Errorf("server2.Name = %q, want %q", server2.Name, "server2")
	}
	if server2.Type != "remote" {
		t.Errorf("server2.Type = %q, want %q", server2.Type, "remote")
	}
	if server2.Enabled {
		t.Error("server2.Enabled = true, want false")
	}
}

func TestParseMCP_EmptyMCP(t *testing.T) {
	data := []byte(`{"mcp": {}}`)

	_, mcpList := parseMCP(data)

	if len(mcpList) != 0 {
		t.Errorf("parseMCP() with empty mcp = %d items, want 0", len(mcpList))
	}
}

func TestParseMCP_NoMCPKey(t *testing.T) {
	mcpMap, mcpList := parseMCP([]byte(`{"other": "value"}`))

	if mcpMap != nil {
		t.Error("parseMCP() with no mcp key returned non-nil map")
	}
	if len(mcpList) != 0 {
		t.Error("parseMCP() with no mcp key returned non-empty list")
	}
}

func TestParseMCP_MissingType(t *testing.T) {
	mcpMap, mcpList := parseMCP([]byte(`{
		"mcp": {
			"server1": {
				"command": "test"
			}
		}
	}`))

	if len(mcpList) != 1 {
		t.Fatalf("parseMCP() list length = %d, want 1", len(mcpList))
	}

	server1 := mcpList[0]
	if server1.Type != "unknown" {
		t.Errorf("server1.Type = %q, want 'unknown'", server1.Type)
	}
	if server1.Enabled {
		t.Error("server1.Enabled = true, want false (default)")
	}
	if mcpMap == nil {
		t.Error("mcpMap should not be nil")
	}
}

func TestParseMCP_MissingEnabled(t *testing.T) {
	mcpMap, mcpList := parseMCP([]byte(`{
		"mcp": {
			"server1": {
				"type": "local"
			}
		}
	}`))

	if len(mcpList) != 1 {
		t.Fatalf("parseMCP() list length = %d, want 1", len(mcpList))
	}

	server1 := mcpList[0]
	if server1.Enabled {
		t.Error("server1.Enabled = false, want false (default)")
	}
	if mcpMap == nil {
		t.Error("mcpMap should not be nil")
	}
}

func TestParseMCP_JSONFiles(t *testing.T) {
	testDir := "testdata"

	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Skipf("testdata directory not available: %v", err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		t.Run(f.Name(), func(t *testing.T) {
			filePath := filepath.Join(testDir, f.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("Failed to read test file: %v", err)
			}

			mcpMap, mcpList := parseMCP(data)

			if mcpMap == nil || mcpList == nil {
				t.Skip("Test file has no mcp data (expected for some test files)")
				return
			}

			for _, entity := range mcpList {
				t.Logf("  - %s: type=%s, enabled=%v", entity.Name, entity.Type, entity.Enabled)
			}
		})
	}
}

func TestModel_updateRawJSON_Enable(t *testing.T) {
	mcpList := []MCPEntity{
		{Name: "server1", Enabled: true, Raw: json.RawMessage(`{"type":"local","enabled":false}`)},
	}

	mcpRawMap := map[string]json.RawMessage{
		"server1": json.RawMessage(`{"type":"local","enabled":false}`),
	}

	model := Model{mcpList: mcpList, mcpRawMap: mcpRawMap}

	model.updateRawJSON(0)

	updatedRaw := model.mcpRawMap["server1"]
	var entity map[string]interface{}
	json.Unmarshal(updatedRaw, &entity)

	enabled, ok := entity["enabled"].(bool)
	if !ok || !enabled {
		t.Error("server1 should be enabled in raw JSON after update")
	}
}

func TestModel_updateRawJSON_Disable(t *testing.T) {
	mcpList := []MCPEntity{
		{Name: "server1", Enabled: false, Raw: json.RawMessage(`{"type":"local","enabled":true}`)},
	}

	mcpRawMap := map[string]json.RawMessage{
		"server1": json.RawMessage(`{"type":"local","enabled":true}`),
	}

	model := Model{mcpList: mcpList, mcpRawMap: mcpRawMap}

	model.updateRawJSON(0)

	updatedRaw := model.mcpRawMap["server1"]
	var entity map[string]interface{}
	json.Unmarshal(updatedRaw, &entity)

	enabled, ok := entity["enabled"].(bool)
	if !ok || enabled {
		t.Error("server1 should be disabled in raw JSON after update")
	}
}

func TestModel_saveConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mcpm-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.json")
	configData := json.RawMessage(`{"mcp":{"server1":{}}}`)

	model := Model{
		mcpList: []MCPEntity{
			{Name: "server1", Enabled: true, Raw: json.RawMessage(`{"type":"local","enabled":true}`)},
		},
		mcpRawMap: map[string]json.RawMessage{
			"server1": json.RawMessage(`{"type":"local","enabled":true}`),
		},
		configData: configData,
	}

	originalGetConfigPath := getConfigPathFunc
	getConfigPathFunc = func() string {
		return configPath
	}
	defer func() {
		getConfigPathFunc = originalGetConfigPath
	}()

	err = model.saveConfig()
	if err != nil {
		t.Fatalf("saveConfig() failed: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read written config: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("Written config is invalid JSON: %v", err)
	}

	mcp, ok := result["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp key not found in written config")
	}

	server1, ok := mcp["server1"].(map[string]interface{})
	if !ok {
		t.Fatal("server1 not found in mcp")
	}

	enabled, ok := server1["enabled"].(bool)
	if !ok || !enabled {
		t.Error("server1.enabled should be true in saved config")
	}
}

func TestModel_saveConfig_InvalidJSON(t *testing.T) {
	model := Model{
		configData: json.RawMessage(`invalid json`),
	}

	err := model.saveConfig()
	if err == nil {
		t.Error("saveConfig() should fail with invalid JSON")
	}
}

func TestModel_View_EmptyList(t *testing.T) {
	model := Model{mcpList: []MCPEntity{}}

	view := model.View()
	if !strings.Contains(view, "No MCP entries found") {
		t.Error("View should show 'No MCP entries found' message")
	}
}

func TestModel_View_WithItems(t *testing.T) {
	mcpList := []MCPEntity{
		{Name: "server1", Type: "local", Enabled: true},
		{Name: "server2", Type: "remote", Enabled: false},
	}

	model := Model{mcpList: mcpList, index: 0}

	view := model.View()
	if !strings.Contains(view, "server1") {
		t.Error("View should contain server1")
	}
	if !strings.Contains(view, "server2") {
		t.Error("View should contain server2")
	}
}

func TestModel_View_CommandMode(t *testing.T) {
	model := Model{mcpList: []MCPEntity{{Name: "test"}}, commandMode: true, commandInput: "w"}

	view := model.View()
	if !strings.Contains(view, "Commands:") {
		t.Error("View should show command help in command mode")
	}
}

func TestModel_View_StatusMessage(t *testing.T) {
	model := Model{mcpList: []MCPEntity{{Name: "test"}}, statusMsg: "Custom status"}

	view := model.View()
	if !strings.Contains(view, "Custom status") {
		t.Error("View should contain status message")
	}
}

func TestModel_View_Navigate(t *testing.T) {
	mcpList := []MCPEntity{
		{Name: "server1", Type: "local", Enabled: true},
		{Name: "server2", Type: "remote", Enabled: false},
	}

	model := Model{mcpList: mcpList, index: 1}

	view := model.View()
	if !strings.Contains(view, "server1") {
		t.Error("View should contain server1")
	}
	if !strings.Contains(view, "server2") {
		t.Error("View should contain server2")
	}
	if !strings.Contains(view, "NORMAL") {
		t.Error("View should show NORMAL mode")
	}
}

func TestMCPEntity(t *testing.T) {
	entity := MCPEntity{
		Name:    "test-server",
		Type:    "local",
		Enabled: true,
		Raw:     json.RawMessage(`{"type":"local","enabled":true}`),
	}

	if entity.Name != "test-server" {
		t.Errorf("entity.Name = %q, want 'test-server'", entity.Name)
	}
	if entity.Type != "local" {
		t.Errorf("entity.Type = %q, want 'local'", entity.Type)
	}
	if !entity.Enabled {
		t.Error("entity.Enabled should be true")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
