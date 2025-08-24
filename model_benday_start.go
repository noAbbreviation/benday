package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type bendayStartModel struct {
	selected      int
	selectingFile bool
	filePicker    filepicker.Model
}

func newBendayStartModel() *bendayStartModel {
	newModel := bendayStartModel{}
	newModel.filePicker = newModel.newFilePicker()

	return &newModel
}

func (_ *bendayStartModel) newFilePicker() filepicker.Model {
	filePicker := filepicker.New()
	filePicker.AllowedTypes = []string{".by.png"}
	filePicker.AutoHeight = false
	filePicker.SetHeight(10)
	filePicker.ShowPermissions = false
	filePicker.CurrentDirectory, _ = os.Getwd()

	return filePicker
}

func (m *bendayStartModel) Init() tea.Cmd {
	return m.filePicker.Init()
}

func (m *bendayStartModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.selectingFile {
				m.selectingFile = false
				m.filePicker = m.newFilePicker()

				return m, m.filePicker.Init()
			}

			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.filePicker, cmd = m.filePicker.Update(msg)

	if didSelect, filePath := m.filePicker.DidSelectFile(msg); didSelect {
		newPreview := newPreviewArtModel(filePath)
		return newPreview, newPreview.Init()
	}

	if m.selectingFile {
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down", "ctrl+n", "j":
			m.selected = (m.selected + 1) % 3

		case "shift+tab", "up", "ctrl+p", "k":
			m.selected -= 1

			if m.selected < 0 {
				m.selected = 2
			}

		case "enter":
			if m.selected == 0 {
				newModel := newCreateCanvasModel()
				return newModel, newModel.Init()
			}

			if m.selected == 2 {
				return m, tea.Quit
			}

			m.selectingFile = true
			return m, nil
		}
	}

	return m, cmd
}

func (m *bendayStartModel) View() string {
	if m.selectingFile {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			m.filePicker.View(),
			"",
			"(esc to go back, up/down to select file, left/backspace to go back one directory)",
			fmt.Sprintf("path: \"%v\"", m.filePicker.CurrentDirectory),
		)
	}

	options := [3]string{
		"Create a new file",
		"View a benday png",
		"Exit",
	}

	for i, option := range options {
		selectedStr := " "
		if m.selected == i {
			selectedStr = "+"
		}

		options[i] = fmt.Sprintf("  [%v] %v", selectedStr, option)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		"  Benday (github.com/noAbbreviation/benday)",
		"",
		options[0],
		options[1],
		options[2],
		"",
		"(up/down to select, enter to confirm, esc/ctrl-c to exit program)",
		"",
	)
}
