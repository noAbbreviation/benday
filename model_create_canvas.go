package main

import (
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"image/color"
	"image/png"
)

var (
	Black   = "0"
	Red     = "1"
	Green   = "2"
	Yellow  = "3"
	Blue    = "4"
	Magenta = "5"
	Cyan    = "6"
	White   = "7"

	BrightBlack   = "8"
	BrightRed     = "9"
	BrightGreen   = "10"
	BrightYellow  = "11"
	BrightBlue    = "12"
	BrightMagenta = "13"
	BrightCyan    = "14"
	BrightWhite   = "15"
)

var (
	NotAWholeNumberError    = errors.New("Number must be greater than zero.")
	NotAPositiveNumberError = errors.New("Number must be a positive number.")
	EmptyFileNameError      = errors.New("Filename is empty.")
)

type createCanvasModel struct {
	inputs  *[5]textinput.Model
	focused int
	err     error

	showConfirmPrompt bool
}

const (
	brailleWInputC = iota
	brailleHInputC
	paddingXInputC
	paddingYInputC
	fileNameInputC
)

func newCreateCanvasModel() *createCanvasModel {
	inputs := [5]textinput.Model{}

	inputs[brailleWInputC] = textinput.New()
	inputs[brailleWInputC].Placeholder = ""
	inputs[brailleWInputC].Focus()
	inputs[brailleWInputC].CharLimit = 5
	inputs[brailleWInputC].Width = 7
	inputs[brailleWInputC].Prompt = ""
	inputs[brailleWInputC].Validate = isWholeNumber
	inputs[brailleWInputC].SetValue("")

	inputs[brailleHInputC] = textinput.New()
	inputs[brailleHInputC].Placeholder = ""
	inputs[brailleHInputC].CharLimit = 5
	inputs[brailleHInputC].Width = 7
	inputs[brailleHInputC].Prompt = ""
	inputs[brailleHInputC].Validate = isWholeNumber

	inputs[paddingXInputC] = textinput.New()
	inputs[paddingXInputC].Placeholder = ""
	inputs[paddingXInputC].CharLimit = 5
	inputs[paddingXInputC].Width = 7
	inputs[paddingXInputC].Prompt = ""
	inputs[paddingXInputC].Validate = isValidPadding
	inputs[paddingXInputC].SetValue("0")

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

	return &createCanvasModel{
		inputs: &inputs,
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

func (m *createCanvasModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *createCanvasModel) View() string {
	promptText := ""
	if m.showConfirmPrompt {
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
				errorMessage = fmt.Sprint(modelError)
			}

			errorPrompt := [...]string{
				"Cannot proceed with file creation.",
				errorMessage,
				"",
				"(press any key to go back, ctrl+c to cancel)",
			}
			promptText = strings.Join(errorPrompt[:], "\n")
		} else {
			prompt := [...]string{
				"  Are you sure you want to create this file?",
				fmt.Sprintf("  \"%v\"", m.fileName()),
				"",
				"([Y]es, [N]o / [C]ancel, [B]ack)",
			}
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

	canvasForm := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("%v Width(in braille characters): %s", valid[brailleWInputC], m.inputs[brailleWInputC].View()),
		"",
		fmt.Sprintf("%v Height(in braille characters): %s", valid[brailleHInputC], m.inputs[brailleHInputC].View()),
		"",
		fmt.Sprintf("%v Image padding X(in braille dots): %s", valid[paddingXInputC], m.inputs[paddingXInputC].View()),
		"",
		fmt.Sprintf("%v Image padding Y(in braille dots): %s", valid[paddingYInputC], m.inputs[paddingYInputC].View()),
		"",
		fmt.Sprintf("%v File name prefix: %s", valid[fileNameInputC], m.inputs[fileNameInputC].View()),
	)

	canvasPreview := lipgloss.JoinHorizontal(lipgloss.Center, m.previewCanvas(), " ", canvasForm)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"Generate a new canvas image:",
		"",
		canvasPreview,
		"",
		promptText,
	)
}

func (m createCanvasModel) previewCanvas() string {
	var brailleCharsW int
	var brailleCharsH int

	if err := m.inputs[brailleWInputC].Err; err != nil {
		brailleCharsW = 0
	} else {
		brailleCharsW, _ = strconv.Atoi(m.inputs[brailleWInputC].Value())
	}

	if err := m.inputs[brailleHInputC].Err; err != nil {
		brailleCharsH = 0
	} else {
		brailleCharsH, _ = strconv.Atoi(m.inputs[brailleHInputC].Value())
	}

	if brailleCharsW == 0 && brailleCharsH == 0 {
		return "xxxxx\nxxxxx\nxxxxx\nxxxxx\nxxxxx"
	}

	builder := strings.Builder{}

	if brailleCharsW == 0 {
		for range brailleCharsH - 1 {
			builder.WriteRune('x')
			builder.WriteRune('\n')
		}

		builder.WriteRune('x')
		return builder.String()
	}

	if brailleCharsH == 0 {
		for range brailleCharsW {
			builder.WriteRune('x')
		}

		return builder.String()
	}

	{
		for range brailleCharsH - 1 {
			for range brailleCharsW {
				builder.WriteRune('⣿')
			}
			builder.WriteRune('\n')
		}

		for range brailleCharsW {
			builder.WriteRune('⣿')
		}

		return builder.String()
	}
}

func (m *createCanvasModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

				_value := m.inputs[m.focused].Value()
				m.inputs[m.focused].SetValue(_value)
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

// TODO: Could be wrapping utils
func (m *createCanvasModel) prevItem() {
	m.focused -= 1

	if m.focused < 0 {
		m.focused = len(m.inputs) - 1
	}
}

func (m *createCanvasModel) nextItem() {
	m.focused = (m.focused + 1) % (len(m.inputs))
}

func (m createCanvasModel) createFile() error {
	fileName := m.fileName()

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf(
			"Error creating the file: \"%v\" may have illegal characters.", fileName,
		)
	}

	defer file.Close()

	if err = m.inputs[brailleWInputC].Err; err != nil {
		return fmt.Errorf("Invalid input on width: %v", err)
	}

	if err = m.inputs[brailleHInputC].Err; err != nil {
		return fmt.Errorf("Invalid input on height: %v", err)
	}

	if err = m.inputs[paddingXInputC].Err; err != nil {
		return fmt.Errorf("Invalid input on paddingX: %v", err)
	}

	if err = m.inputs[paddingYInputC].Err; err != nil {
		return fmt.Errorf("Invalid input on paddingY: %v", err)
	}

	if err = m.inputs[fileNameInputC].Err; err != nil {
		return fmt.Errorf("Invalid input on file name prefix: %v", err)
	}

	brailleCharsW, _ := strconv.Atoi(m.inputs[brailleWInputC].Value())
	brailleCharsH, _ := strconv.Atoi(m.inputs[brailleHInputC].Value())
	paddingX, _ := strconv.Atoi(m.inputs[paddingXInputC].Value())
	paddingY, _ := strconv.Atoi(m.inputs[paddingYInputC].Value())

	imageWidth := brailleCharsW * (paddingX + BRAILLE_WIDTH)
	imageHeight := brailleCharsH * (paddingY + BRAILLE_HEIGHT)

	img := image.NewNRGBA(image.Rect(0, 0, imageWidth, imageHeight))

	for y := range imageHeight {
		for x := range imageWidth {
			img.Set(x, y, color.Transparent)
		}
	}

	colorGray := color.Gray{0xcc}
	paintWhiteFlagger := true

	for bigYOff := 0; bigYOff < imageHeight; bigYOff += paddingY + BRAILLE_HEIGHT {
		_paintWhite := paintWhiteFlagger

		for bigXOff := 0; bigXOff < imageWidth; bigXOff += paddingX + BRAILLE_WIDTH {
			for charYOff := 0; charYOff < BRAILLE_HEIGHT; charYOff += 1 {
				for charXOff := 0; charXOff < BRAILLE_WIDTH; charXOff += 1 {
					x := bigXOff + charXOff
					y := bigYOff + charYOff

					if _paintWhite {
						img.Set(x, y, color.White)
					} else {
						img.Set(x, y, colorGray)
					}
				}
			}

			_paintWhite = !_paintWhite
		}

		paintWhiteFlagger = !paintWhiteFlagger
	}

	err = png.Encode(io.Writer(file), img)
	if err != nil {
		return err
	}

	return nil
}
