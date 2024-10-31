package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DeleteModel struct {
	config    *MydConfig
	paths     []string
	cursor    int
	selected  map[int]bool
	quitting  bool
}

func NewDeleteModel(config *MydConfig) *DeleteModel {
	paths := loadPaths(config)
	return &DeleteModel{
		config:    config,
		paths:     paths,
		selected:  make(map[int]bool),
		quitting:  false,
	}
}

func loadPaths(config *MydConfig) []string {
	uploadListPath := filepath.Join(os.ExpandEnv(config.StoragePath), "toupload.txt")
	data, err := os.ReadFile(uploadListPath)
	if err != nil {
		return []string{}
	}

	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

func (m *DeleteModel) Init() tea.Cmd {
	return nil
}

func (m *DeleteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.paths)-1 {
				m.cursor++
			}
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
			return m, nil
		case "enter":
			if len(m.selected) > 0 {
				return m, tea.Sequence(
					m.deleteSelected,
					func() tea.Msg { return nil },
				)
			}
		}
	case error:
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *DeleteModel) deleteSelected() tea.Msg {
	// Create a new slice without the selected paths
	var newPaths []string
	for i, path := range m.paths {
		if !m.selected[i] {
			newPaths = append(newPaths, path)
		}
	}

	// Write the new paths back to toupload.txt
	uploadListPath := filepath.Join(os.ExpandEnv(m.config.StoragePath), "toupload.txt")
	content := strings.Join(newPaths, "\n")
	if len(newPaths) > 0 {
		content += "\n"
	}
	
	if err := os.WriteFile(uploadListPath, []byte(content), 0644); err != nil {
		return err
	}

	// Update the model's paths
	m.paths = newPaths
	m.selected = make(map[int]bool)
	if m.cursor >= len(m.paths) {
		m.cursor = len(m.paths) - 1
	}
	return nil
}

func (m *DeleteModel) View() string {
	if m.quitting {
		return ""
	}

	if len(m.paths) == 0 {
		return "No paths are being tracked\n\nPress q to quit"
	}

	s := "Select paths to delete (space to select, enter to delete):\n\n"

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	selectedStyle := style.Copy().Bold(true)

	for i, path := range m.paths {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		if m.selected[i] {
			checked = "x"
		}

		line := fmt.Sprintf("%s [%s] %s", cursor, checked, path)
		if m.cursor == i {
			s += selectedStyle.Render(line) + "\n"
		} else {
			s += line + "\n"
		}
	}

	s += "\nPress q to quit\n"
	return s
} 