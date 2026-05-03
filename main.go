package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var getConfigPathFunc = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/home/steam/.config/opencode/opencode.json"
	}
	return home + "/.config/opencode/opencode.json"
}

func getConfigPath() string {
	return getConfigPathFunc()
}

var (
	selectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("#333333")).Foreground(lipgloss.Color("#ffffff"))
	enabledStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	disabledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
	typeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffff00"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
)

type Model struct {
	mcpList      []MCPEntity
	mcpRawMap    map[string]json.RawMessage
	index        int
	quitting     bool
	statusMsg    string
	statusTime   int
	commandMode  bool
	commandInput string
	configData   json.RawMessage
}

type MCPEntity struct {
	Name    string
	Type    string
	Enabled bool
	Raw     json.RawMessage
}

func NewModel() Model {
	data, err := os.ReadFile(getConfigPath())
	if err != nil {
		return Model{
			statusMsg:  fmt.Sprintf("Error reading config: %v", err),
			statusTime: 60,
		}
	}

	mcpMap, mcpList := parseMCP(data)

	return Model{
		mcpRawMap:  mcpMap,
		mcpList:    mcpList,
		configData: data,
		statusMsg:  "Load successful",
		statusTime: 60,
	}
}

func parseMCP(data []byte) (map[string]json.RawMessage, []MCPEntity) {
	var config map[string]json.RawMessage
	json.Unmarshal(data, &config)

	var mcpRaw map[string]json.RawMessage
	if mcpData, ok := config["mcp"]; ok {
		json.Unmarshal(mcpData, &mcpRaw)
	} else {
		return nil, nil
	}

	var names []string
	for name := range mcpRaw {
		names = append(names, name)
	}
	sort.Strings(names)

	var mcpList []MCPEntity
	for _, name := range names {
		entity := MCPEntity{
			Name: name,
			Raw:  mcpRaw[name],
		}

		var typeField map[string]json.RawMessage
		json.Unmarshal(mcpRaw[name], &typeField)
		if t, ok := typeField["type"]; ok {
			entity.Type = strings.Trim(string(t), `"`)
		} else {
			entity.Type = "unknown"
		}

		if e, ok := typeField["enabled"]; ok {
			json.Unmarshal(e, &entity.Enabled)
		} else {
			entity.Enabled = false
		}

		mcpList = append(mcpList, entity)
	}

	return mcpRaw, mcpList
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.commandMode {
			return m.handleCommandKey(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			m.commandMode = false
			m.commandInput = ""
			m.statusMsg = "Command mode cancelled"
			m.statusTime = 60
			return m, nil
		case "up", "k":
			if m.index > 0 {
				m.index--
			}
		case "down", "j":
			if m.index < len(m.mcpList)-1 {
				m.index++
			}
		case "left":
			if len(m.mcpList) > 0 {
				m.mcpList[m.index].Enabled = false
				m.updateRawJSON(m.index)
				m.statusMsg = fmt.Sprintf("Disabled %s", m.mcpList[m.index].Name)
				m.statusTime = 60
			}
		case "right":
			if len(m.mcpList) > 0 {
				m.mcpList[m.index].Enabled = true
				m.updateRawJSON(m.index)
				m.statusMsg = fmt.Sprintf("Enabled %s", m.mcpList[m.index].Name)
				m.statusTime = 60
			}
		case ":":
			m.commandMode = true
			m.commandInput = ""
			return m, nil
		}

	case tea.WindowSizeMsg:
		return m, nil
	}

	if m.statusTime > 0 {
		m.statusTime--
	}

	return m, nil
}

func (m Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		cmd := strings.TrimSpace(m.commandInput)
		m.commandMode = false
		m.commandInput = ""

		switch cmd {
		case "w":
			if err := m.saveConfig(); err != nil {
				m.statusMsg = fmt.Sprintf("Save error: %v", err)
			} else {
				m.statusMsg = "Config saved successfully"
			}
			m.statusTime = 60
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "wq":
			if err := m.saveConfig(); err != nil {
				m.statusMsg = fmt.Sprintf("Save error: %v", err)
			} else {
				m.statusMsg = "Config saved and exiting"
			}
			m.quitting = true
			return m, tea.Quit
		case "q!":
			m.quitting = true
			return m, tea.Quit
		case "":
			m.statusMsg = "No command"
		default:
			m.statusMsg = fmt.Sprintf("Unknown command: %s (press Esc to cancel)", cmd)
		}
		m.statusTime = 60
		return m, nil

	case "ctrl+c":
		m.commandMode = false
		m.commandInput = ""
		m.statusMsg = "Command cancelled"
		m.statusTime = 60
		return m, nil

	case "escape":
		m.commandMode = false
		m.commandInput = ""
		m.statusMsg = "Command mode cancelled"
		m.statusTime = 60
		return m, nil

	case "backspace":
		if len(m.commandInput) > 0 {
			m.commandInput = m.commandInput[:len(m.commandInput)-1]
		}
	case "ctrl+h":
		if len(m.commandInput) > 0 {
			m.commandInput = m.commandInput[:len(m.commandInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.commandInput += msg.String()
		}
	}

	return m, nil
}

func (m Model) updateRawJSON(index int) {
	name := m.mcpList[index].Name
	var entity map[string]interface{}
	json.Unmarshal(m.mcpRawMap[name], &entity)
	entity["enabled"] = m.mcpList[index].Enabled

	newRaw, _ := json.Marshal(entity)
	m.mcpRawMap[name] = newRaw
}

func (m Model) saveConfig() error {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(m.configData, &config); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	newMCP, err := json.Marshal(m.mcpRawMap)
	if err != nil {
		return fmt.Errorf("failed to marshal MCP: %v", err)
	}
	config["mcp"] = newMCP

	updatedJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Validate by unmarshaling
	var validate map[string]interface{}
	if err := json.Unmarshal(updatedJSON, &validate); err != nil {
		return fmt.Errorf("JSON validation failed: %v", err)
	}

	if err := os.WriteFile(getConfigPath(), updatedJSON, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString("MCP Manager - opencode.json\n\n")

	if len(m.mcpList) == 0 {
		b.WriteString("No MCP entries found\n")
		b.WriteString("\nPress q to quit\n")
		return b.String()
	}

	for i, entity := range m.mcpList {
		var indicator string
		var status string

		if entity.Enabled {
			indicator = "✓"
			status = enabledStyle.Render("enabled")
		} else {
			indicator = "✗"
			status = disabledStyle.Render("disabled")
		}

		line := fmt.Sprintf("  %s %s (%s) - %s",
			indicator,
			entity.Name,
			typeStyle.Render(entity.Type),
			status)

		if i == m.index {
			line = selectedStyle.Render(line)
		}

		b.WriteString(line + "\n")
	}

	b.WriteString("\n")

	if m.commandMode {
		b.WriteString(fmt.Sprintf("%s%s", statusStyle.Render(":"), statusStyle.Render(m.commandInput)))
		b.WriteString("\n")
		b.WriteString("Commands: :w (save), :q (quit), :wq (save+quit), :q! (force quit)\n")
	} else {
		b.WriteString(statusStyle.Render("NORMAL") + " MODE ")
		b.WriteString("\n")
		b.WriteString("↑/↓ or j/k: navigate | ←: disable | →: enable | : command mode | q: quit\n")
	}

	if m.statusMsg != "" {
		b.WriteString("\n" + m.statusMsg + "\n")
	}

	return b.String()
}

func main() {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
