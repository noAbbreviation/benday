package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"errors"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	NotAWholeNumberError    = errors.New("Number must be greater than zero.")
	NotAPositiveNumberError = errors.New("Number must be a positive number.")
)

type canvasModel struct {
	pixels   [][]bool
	fileName string
}

func newCanvas() canvasModel {
	return canvasModel{
		pixels: [][]bool{},
	}
}

func (m canvasModel) Init() tea.Cmd {
	return tea.SetWindowTitle("benday")
}

func (m canvasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "e":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m canvasModel) View() string {
	render := []string{
		"Hello world!",
		renderBraille(m.pixels),
		"(Ctrl-C or q to quit)",
	}

	return strings.Join(render, "\n")
}

func renderBraille(_ [][]bool) string {
	result := ""
	for i, braille := range brailleLookup {
		if i%32 == 0 {
			result += "\n"
		}

		result += string(braille)
	}

	return result
}

type createCanvasModel struct {
	inputs  [5]textinput.Model
	focused int
	err     error

	showConfirmPrompt bool
}

const (
	widthInputC = iota
	heightInputC
	paddingXInputC
	paddingYInputC
	fileNameInputC
)

func newCreateCanvasModel() createCanvasModel {
	inputs := [5]textinput.Model{}

	inputs[widthInputC] = textinput.New()
	inputs[widthInputC].Placeholder = ""
	inputs[widthInputC].Focus()
	inputs[widthInputC].CharLimit = 5
	inputs[widthInputC].Width = 7
	inputs[widthInputC].Prompt = ""
	inputs[widthInputC].Validate = isWholeNumber

	inputs[heightInputC] = textinput.New()
	inputs[heightInputC].Placeholder = ""
	inputs[heightInputC].CharLimit = 5
	inputs[heightInputC].Width = 7
	inputs[heightInputC].Prompt = ""
	inputs[heightInputC].Validate = isWholeNumber

	inputs[paddingXInputC] = textinput.New()
	inputs[paddingXInputC].Placeholder = ""
	inputs[paddingXInputC].CharLimit = 5
	inputs[paddingXInputC].Width = 7
	inputs[paddingXInputC].Prompt = ""
	inputs[paddingXInputC].Validate = isWholeNumber

	inputs[paddingYInputC] = textinput.New()
	inputs[paddingYInputC].Placeholder = ""
	inputs[paddingYInputC].CharLimit = 5
	inputs[paddingYInputC].Width = 7
	inputs[paddingYInputC].Prompt = ""
	inputs[paddingYInputC].Validate = isValidPadding

	inputs[fileNameInputC] = textinput.New()
	inputs[fileNameInputC].Placeholder = "newCanvas"
	inputs[fileNameInputC].CharLimit = 64
	inputs[fileNameInputC].Width = 64
	inputs[fileNameInputC].Prompt = ""
	inputs[fileNameInputC].Validate = isValidPadding

	return createCanvasModel{
		inputs: inputs,
	}
}

func isWholeNumber(s string) error {
	num, err := strconv.Atoi(s)
	if err != nil {
		return NotAPositiveNumberError
	}

	if num == 0 || num < 0 {
		return NotAWholeNumberError
	}

	return nil
}

func isValidPadding(s string) error {
	num, err := strconv.Atoi(s)
	if err != nil {
		return NotAWholeNumberError
	}

	if num == 0 || num < 0 {
		return NotAWholeNumberError
	}

	return nil
}

func (m createCanvasModel) fileName() string {
	fileName := fmt.Sprintf(
		"%v.%vx%v.by.png",
		m.inputs[fileNameInputC].Value(),
		m.inputs[paddingXInputC].Value(),
		m.inputs[paddingYInputC].Value(),
	)

	return fileName
}

func (m createCanvasModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m createCanvasModel) View() string {
	promptText := ""
	if m.showConfirmPrompt {
		prompt := [...]string{
			"",
			"Are you sure you want to create this file?",
			fmt.Sprintf("\"%v\"", m.fileName()),
			"",
			"([y]es, [n]o, [c]ancel, [b]ack)",
		}
		promptText = strings.Join(prompt[:], "\n")
	}

	result := [...]string{
		"Create new canvas image:",
		"*--",
		fmt.Sprintf("| Width(in characters): %s", m.inputs[widthInputC].View()),
		fmt.Sprintf("| Height(in characters): %s", m.inputs[heightInputC].View()),
		fmt.Sprintf("| Padding X(in braille dots): %s", m.inputs[paddingXInputC].View()),
		fmt.Sprintf("| Padding Y(in braille dots): %s", m.inputs[paddingYInputC].View()),
		fmt.Sprintf("| File name prefix: %s", m.inputs[fileNameInputC].View()),
		"*--",
		promptText,
	}
	return strings.Join(result[:], "\n")
}

func (m createCanvasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	if keyMsg, ok := msg.(tea.KeyMsg); ok && (keyMsg.String() == "ctrl+c" || keyMsg.String() == "esc") {
		return m, tea.Quit
	}

	if m.showConfirmPrompt {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "enter":
				//  TODO: Create a new file from here
				return m, tea.Quit
			case "n", "b":
				m.showConfirmPrompt = false
				return m, nil
			case "c":
				return m, tea.Quit
			}
		}

	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		{
			switch msg.Type {
			case tea.KeyEnter:
				if m.focused == len(m.inputs)-1 {
					m.showConfirmPrompt = true
				} else {
					m.nextItem()
				}
			case tea.KeyShiftTab, tea.KeyCtrlP, tea.KeyUp:
				m.prevItem()
			case tea.KeyTab, tea.KeyCtrlN, tea.KeyDown:
				m.nextItem()
			}

			for i := range m.inputs {
				m.inputs[i].Blur()
			}
			m.inputs[m.focused].Focus()
		}

	case error:
		m.err = msg
		return m, nil
	}

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m *createCanvasModel) prevItem() {
	m.focused -= 1

	if m.focused < 0 {
		m.focused = len(m.inputs) - 1
	}
}

func (m *createCanvasModel) nextItem() {
	m.focused = (m.focused + 1) % (len(m.inputs))
}

func main() {
	model := tea.Model(newCanvas())

	if len(os.Args) < 2 {
		model = newCreateCanvasModel()
	}

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
