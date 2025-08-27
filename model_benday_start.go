package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type bendayStartModel struct {
	focusedOpt    int
	selectingFile bool
	importingFile bool

	filePicker filepicker.Model
	err        error
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
			if m.selectingFile || m.importingFile {
				m.selectingFile = false
				m.importingFile = false

				m.filePicker = m.newFilePicker()
				return m, m.filePicker.Init()
			}

			return m, tea.Quit
		}
	}

	if m.selectingFile || m.importingFile {
		if m.err != nil {
			if _, isKeyMsg := msg.(tea.KeyMsg); isKeyMsg {
				m.err = nil
				return m, nil
			}

			return m, nil
		}

		var cmd tea.Cmd
		m.filePicker, cmd = m.filePicker.Update(msg)

		if didSelect, filePath := m.filePicker.DidSelectFile(msg); didSelect {
			if m.selectingFile {
				newPreview := newPreviewArtModel(filePath)
				return newPreview, newPreview.Init()
			}

			if m.importingFile {
				file, err := os.Open(filePath)
				if err != nil {
					m.err = FileDoesNotExistError
					return m, nil
				}

				defer file.Close()

				pixels, err := importPixelData(file)
				if err != nil {
					m.err = err
					return m, nil
				}

				importModel := newImportCanvasModel(pixels)
				return importModel, importModel.Init()
			}
		}

		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down", "ctrl+n", "j":
			m.focusedOpt = (m.focusedOpt + 1) % 4

		case "shift+tab", "up", "ctrl+p", "k":
			m.focusedOpt -= 1

			if m.focusedOpt < 0 {
				m.focusedOpt = 3
			}

		case "enter":
			switch m.focusedOpt {
			case 0:
				newModel := newCreateCanvasModel()
				return newModel, newModel.Init()
			case 1:
				m.selectingFile = true
				m.filePicker.AllowedTypes = []string{".by.png"}

				return m, m.filePicker.Init()
			case 2:
				m.importingFile = true
				m.filePicker.AllowedTypes = []string{".txt"}

				return m, m.filePicker.Init()
			default:
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func importPixelData(brailleAsciiFile *os.File) ([][]rune, error) {
	pixels := [][]rune{}
	scanner := bufio.NewScanner(brailleAsciiFile)

	maxLen := -1
	for scanner.Scan() {
		brailleLine := scanner.Text()
		brailleLine = strings.Map(func(r rune) rune {
			if isBraille(r) {
				return r
			}

			if r == ' ' {
				return r
			}

			return -1
		}, brailleLine)

		pixelLine := []rune(brailleLine)
		pixels = append(pixels, pixelLine)

		maxLen = max(maxLen, len(pixelLine))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(pixels) == 0 {
		return nil, fmt.Errorf("No data received.")
	}

	linesAreEmpty := true
	for _, line := range pixels {
		if len(line) != 0 {
			linesAreEmpty = false
			break
		}
	}

	if linesAreEmpty {
		return nil, fmt.Errorf("No data received.")
	}

	for i := range pixels {
		line := pixels[i]
		for range maxLen - len(line) {
			line = append(line, '⠀')
		}
	}

	return pixels, nil
}

func (m *bendayStartModel) View() string {
	if m.selectingFile || m.importingFile {
		commandText := "previewing file"

		if m.importingFile {
			commandText = "importing file"

			if m.err != nil {
				return lipgloss.JoinVertical(
					lipgloss.Left,
					"",
					"Error importing the braille text file:",
					m.err.Error(),
					"",
					"(import failed) (any key to go back)",
				)
			}
		}

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			m.filePicker.View(),
			"",
			fmt.Sprintf("(%v) (esc to go back, up/down to select file, left/backspace to go back one directory)", commandText),
			fmt.Sprintf("path: \"%v\"", m.filePicker.CurrentDirectory),
		)
	}

	options := [4]string{
		"Create a new file",
		"View a benday png",
		"Import a braille ascii file",
		"Exit",
	}

	for i, option := range options {
		selectedStr := " "
		if m.focusedOpt == i {
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
		options[3],
		"",
		"(up/down to select, enter to confirm, esc/ctrl-c to exit program)",
		"",
	)
}
