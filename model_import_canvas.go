package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type importCanvasModel struct {
	inputs  *[3]textinput.Model
	pixels  [][]rune
	focused int
	err     error

	showConfirmPrompt bool
	_fromArgs         bool
}

const (
	paddingXInputI = iota
	paddingYInputI = iota
	fileNameInputI = iota
)

func newImportCanvasModel(pixels [][]rune) *importCanvasModel {
	inputs := [3]textinput.Model{}

	inputs[paddingXInputI] = textinput.New()
	inputs[paddingXInputI].Placeholder = ""
	inputs[paddingXInputI].CharLimit = 2
	inputs[paddingXInputI].Width = 5
	inputs[paddingXInputI].Prompt = ""
	inputs[paddingXInputI].Validate = isValidPadding
	inputs[paddingXInputI].SetValue("0")
	inputs[paddingXInputI].Focus()

	inputs[paddingYInputI] = textinput.New()
	inputs[paddingYInputI].Placeholder = ""
	inputs[paddingYInputI].CharLimit = 2
	inputs[paddingYInputI].Width = 5
	inputs[paddingYInputI].Prompt = ""
	inputs[paddingYInputI].Validate = isValidPadding
	inputs[paddingYInputI].SetValue("2")

	inputs[fileNameInputI] = textinput.New()
	inputs[fileNameInputI].Placeholder = ""
	inputs[fileNameInputI].CharLimit = 64
	inputs[fileNameInputI].Width = 64
	inputs[fileNameInputI].Prompt = ""
	inputs[fileNameInputI].Validate = isValidFileName

	return &importCanvasModel{
		inputs: &inputs,
		pixels: pixels,
		err:    nil,
	}
}

func importCanvasModelFromArgs(pixels [][]rune) *importCanvasModel {
	model := newImportCanvasModel(pixels)
	model._fromArgs = true

	return model
}

func (m *importCanvasModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *importCanvasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.showConfirmPrompt {
				m.showConfirmPrompt = false
				m.inputs[m.focused].Focus()

				return m, nil
			}

			if m._fromArgs {
				return m, tea.Quit
			}

			startingModel := newBendayStartModel()
			return startingModel, startingModel.Init()
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
				m.err = nil

				return m, nil
			}

			return m, nil
		}

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "enter":
				if err := m.createFile(); err != nil {
					m.err = err
					return m, nil
				}

				previewModel := newPreviewArtModel(m.fileName())
				return previewModel, previewModel.Init()
			case "n", "b":
				m.showConfirmPrompt = false
				m.inputs[m.focused].Focus()
				return m, nil
			}

		case error:
			m.err = msg
			return m, nil
		}

		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.focused == len(m.inputs)-1 {
				m.showConfirmPrompt = true
			} else {
				m.focused = (m.focused + 1) % len(m.inputs)
			}
		case tea.KeyShiftTab, tea.KeyCtrlP, tea.KeyUp:
			m.focused -= 1

			if m.focused < 0 {
				m.focused = len(m.inputs) - 1
			}
		case tea.KeyTab, tea.KeyCtrlN, tea.KeyDown:
			m.focused = (m.focused + 1) % len(m.inputs)
		}

		for i := range m.inputs {
			m.inputs[i].Blur()
		}

		if !m.showConfirmPrompt {
			m.inputs[m.focused].Focus()

			_value := m.inputs[m.focused].Value()
			m.inputs[m.focused].SetValue(_value)
		}
	}

	cmds := [len(m.inputs)]tea.Cmd{}
	for i, input := range m.inputs {
		m.inputs[i], cmds[i] = input.Update(msg)
	}

	return m, tea.Batch(cmds[:]...)
}

func (m *importCanvasModel) fileName() string {
	fileName := fmt.Sprintf(
		"%v.%vx%v.by.png",
		m.inputs[fileNameInputI].Value(),
		m.inputs[paddingXInputI].Value(),
		m.inputs[paddingYInputI].Value(),
	)

	return fileName
}

func (m importCanvasModel) createFile() error {
	fileName := m.fileName()

	_, err := os.Stat(fileName)
	if err == nil {
		return fmt.Errorf("File already exists.")
	}

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf(
			"Error creating the file: \"%v\" may have illegal characters.", fileName,
		)
	}

	defer file.Close()

	if err = m.inputs[paddingXInputI].Err; err != nil {
		return fmt.Errorf("Invalid input on paddingX: %v", err)
	}

	if err = m.inputs[paddingYInputI].Err; err != nil {
		return fmt.Errorf("Invalid input on paddingY: %v", err)
	}

	if err = m.inputs[fileNameInputI].Err; err != nil {
		return fmt.Errorf("Invalid input on file name prefix: %v", err)
	}

	paddingX, _ := strconv.Atoi(m.inputs[paddingXInputI].Value())
	paddingY, _ := strconv.Atoi(m.inputs[paddingYInputI].Value())

	charsX := len(m.pixels[0])
	charsY := len(m.pixels)

	imageWidth := charsX * (paddingX + BRAILLE_WIDTH)
	imageHeight := charsY * (paddingY + BRAILLE_HEIGHT)

	img := newCanvasImage(imageWidth, imageHeight, paddingX, paddingY, false).(*image.NRGBA)

	for charY, _line := range m.pixels {
		for charX, charRune := range _line {
			brailleBits := []rune(strconv.FormatUint(uint64(brailleReverseLookup[charRune]), 2))

			for range BRAILLE_WIDTH*BRAILLE_HEIGHT - len(brailleBits) {
				brailleBits = append([]rune{'0'}, brailleBits...)
			}
			slices.Reverse(brailleBits)

			for brailleYOff := range BRAILLE_HEIGHT {
				for brailleXOff := range BRAILLE_WIDTH {
					bitsIdx := brailleYOff*BRAILLE_WIDTH + brailleXOff

					if brailleBits[bitsIdx] != '1' {
						continue
					}

					x := charX*(BRAILLE_WIDTH+paddingX) + brailleXOff
					y := charY*(BRAILLE_HEIGHT+paddingY) + brailleYOff

					colorBlack := color.NRGBA{0x33, 0x33, 0x33, 0xff}
					img.SetNRGBA(x, y, colorBlack)
				}
			}
		}
	}

	encodeErr := png.Encode(file, img)
	return encodeErr
}

func (m *importCanvasModel) promptText() string {
	if !m.showConfirmPrompt {
		if m.focused == len(m.inputs)-1 {
			return "(importing to benday) (enter to continue, up/down to navigate, ctrl-c to exit program, esc to go back)"
		}

		return "(importing to benday) (up/down to navigate, ctrl-c to exit program, esc to go back)"
	}

	hasError := false
	for _, input := range m.inputs {
		if input.Err != nil {
			hasError = true
			break
		}
	}

	if modelError := m.err; hasError || modelError != nil {
		errorMessage := "Fields marked with question marks(?) are invalid."
		if modelError != nil {
			errorMessage = modelError.Error()
		}

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"Cannot proceed with importing file:",
			errorMessage,
			"",
			"(importing failed) (press any key to go back, ctrl-c to exit program)",
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"  Are you sure you want to create this file?",
		fmt.Sprintf("  \"%v\"", m.fileName()),
		"",
		"(importing to benday) (y to confirm, n to go back)",
	)
}

func (m *importCanvasModel) View() string {
	promptText := m.promptText()

	valid := [len(m.inputs)]string{}
	for i, input := range m.inputs {
		if input.Err != nil {
			valid[i] = "?"
		} else {
			valid[i] = ">"
		}
	}

	canvasForm := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("%v Image padding X(in braille dots): %s", valid[paddingXInputI], m.inputs[paddingXInputI].View()),
		"",
		fmt.Sprintf("%v Image padding Y(in braille dots): %s", valid[paddingYInputI], m.inputs[paddingYInputI].View()),
		"",
		fmt.Sprintf("%v File name prefix: %s", valid[fileNameInputI], m.inputs[fileNameInputI].View()),
	)

	previewBuilder := strings.Builder{}
	for _, pixel := range m.pixels[0] {
		previewBuilder.WriteRune(pixel)
	}

	for _, line := range m.pixels[1:] {
		previewBuilder.WriteRune('\n')
		for _, pixel := range line {
			previewBuilder.WriteRune(pixel)
		}
	}

	previewCanvas := lipgloss.JoinHorizontal(
		lipgloss.Center,
		previewBorder.Render(previewBuilder.String()),
		" ",
		canvasForm,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		"Import a braille ascii file:",
		previewCanvas,
		"",
		promptText,
		"",
	)
}
