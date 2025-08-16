package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	NotAWholeNumberError    = errors.New("Number must be greater than zero.")
	NotAPositiveNumberError = errors.New("Number must be a positive number.")
	EmptyFileNameError      = errors.New("Filename is empty.")
)

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
	inputs[paddingXInputC].SetValue("0")
	inputs[paddingXInputC].Validate = isValidPadding

	inputs[paddingYInputC] = textinput.New()
	inputs[paddingYInputC].Placeholder = ""
	inputs[paddingYInputC].CharLimit = 5
	inputs[paddingYInputC].Width = 7
	inputs[paddingYInputC].Prompt = ""
	inputs[paddingYInputC].SetValue("2")
	inputs[paddingYInputC].Validate = isValidPadding

	inputs[fileNameInputC] = textinput.New()
	inputs[fileNameInputC].Placeholder = ""
	inputs[fileNameInputC].CharLimit = 64
	inputs[fileNameInputC].Width = 64
	inputs[fileNameInputC].Prompt = ""
	inputs[fileNameInputC].Validate = isValidFileName

	return createCanvasModel{
		inputs: inputs,
		err:    nil,
	}
}

func isWholeNumber(s string) error {
	if len(s) < 1 {
		return NotAWholeNumberError
	}

	num, err := strconv.Atoi(s)
	if err != nil {
		return NotAWholeNumberError
	}

	if num == 0 || num < 0 {
		return NotAWholeNumberError
	}

	return nil
}

func isValidPadding(s string) error {
	if len(s) < 1 {
		return NotAWholeNumberError
	}

	num, err := strconv.Atoi(s)
	if err != nil {
		return NotAPositiveNumberError
	}

	if num < 0 {
		return NotAPositiveNumberError
	}

	return nil
}

func isValidFileName(s string) error {
	if len(s) < 1 {
		return EmptyFileNameError
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
		hasError := false
		for _, input := range m.inputs {
			if input.Err != nil {
				hasError = true
				break
			}
		}

		if hasError || m.err != nil {
			errorPrompt := [...]string{
				"",
				"Cannot proceed with image creation.",
				"Fields marked with question marks(?) are invalid.",
				"",
				"(press any key to go back)",
			}
			promptText = strings.Join(errorPrompt[:], "\n")
		} else {
			prompt := [...]string{
				"  Are you sure you want to create this file?",
				fmt.Sprintf("  \"%v\"", m.fileName()),
				"",
				"([Y]es, [N]o / [C]ancel, [B]ack)",
			}
			//  TODO: Add dimensions of the new image to the confirm prompt
			promptText = strings.Join(prompt[:], "\n")
		}
	} else if m.focused == len(m.inputs)-1 {
		promptText = "(enter to continue, ctrl-c to cancel)"
	} else {
		promptText = "(ctrl-c to cancel)"
	}

	valid := []string{}
	for _, input := range m.inputs {
		if input.Err != nil {
			valid = append(valid, "?")
		} else {
			valid = append(valid, ">")
		}
	}

	result := [...]string{
		"Create a new canvas image:",
		"",
		fmt.Sprintf("%v Width(in braille characters): %s", valid[widthInputC], m.inputs[widthInputC].View()),
		fmt.Sprintf("%v Height(in braille characters): %s", valid[heightInputC], m.inputs[heightInputC].View()),
		fmt.Sprintf("%v Padding X(in braille dots): %s", valid[paddingXInputC], m.inputs[paddingXInputC].View()),
		fmt.Sprintf("%v Padding Y(in braille dots): %s", valid[paddingYInputC], m.inputs[paddingYInputC].View()),
		fmt.Sprintf("%v File name prefix: %s", valid[fileNameInputC], m.inputs[fileNameInputC].View()),
		"",
		promptText,
	}
	return strings.Join(result[:], "\n")
}

func (m createCanvasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	if m.showConfirmPrompt {
		hasError := false
		for _, input := range m.inputs {
			if input.Err != nil {
				hasError = true
				break
			}
		}

		if hasError || m.err != nil {
			if _, ok := msg.(tea.KeyMsg); ok {
				m.showConfirmPrompt = false
				m.inputs[m.focused].Focus()
				return m, nil
			}

			return m, nil
		}

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "enter":
				//  TODO: Create a new file from here
				return m, tea.Quit
			case "b":
				m.showConfirmPrompt = false
				m.inputs[m.focused].Focus()
				return m, nil
			case "n", "c":
				return m, tea.Quit
			}

		case error:
			m.err = msg
			return m, nil
		}

		return m, nil
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

			if !m.showConfirmPrompt {
				m.inputs[m.focused].Focus()
			}
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
