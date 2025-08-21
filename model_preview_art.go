package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	InvalidFileNameError = decodeError{
		errors.New("Invalid file name. File must end in the form \"*.<pX>x<pY>.by.png\"."),
	}

	alphaThreshold = 0xff / 3
)

type decodeError struct {
	error
}

type silentError struct {
	error
}

type InvalidImgDimensionE struct {
	measure           int
	mustBeDivisibleBy int
	errorOnX          bool
}

func (err InvalidImgDimensionE) Error() string {
	measureName := "width"
	if !err.errorOnX {
		measureName = "height"
	}

	return fmt.Sprintf(
		"Invalid image dimension. Expected %v to be divisible by %v, but is instead %v px.",
		measureName,
		err.mustBeDivisibleBy,
		err.measure,
	)
}

type previewArtModel struct {
	fileName string
	err      error

	watchTicker bool
	unpadded    bool
	pixels      [][]rune

	paddingX, paddingY int

	rOpts resizeOptionStore
}

type resizeOptionStore struct {
	resizing          bool
	showConfirmPrompt bool
	inputs            *[2]textinput.Model
}

func newPreviewArtModel(fileName string) *previewArtModel {
	newModel := &previewArtModel{fileName: fileName}
	pixelData := newModel.GetPixels()

	newModel.pixels = pixelData.pixels
	newModel.err = pixelData.err

	return newModel
}

func (m *previewArtModel) Init() tea.Cmd {
	return func() tea.Msg {
		return m.GetPixels()
	}
}

func (m *previewArtModel) Tick() (*previewArtModel, tea.Cmd) {
	return m, tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return m.GetPixels()
	})
}

type updatePreviewMsg struct {
	err    error
	pixels [][]rune
}

// TODO: Clean options: restore unshaded, non-comment pixels

func (m *previewArtModel) GetPixels() updatePreviewMsg {
	dotChars := strings.Count(m.fileName, ".")
	if dotChars < 3 {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	fileNameInfo := strings.Split(m.fileName, ".")
	slices.Reverse(fileNameInfo)

	if imgExtension := fileNameInfo[0]; imgExtension != "png" {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	if hasBy := fileNameInfo[1] == "by"; !hasBy {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	paddingSpec := fileNameInfo[2]
	if strings.Count(paddingSpec, "x") != 1 {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	paddingSpecSplit := strings.Split(paddingSpec, "x")

	paddingX, err := strconv.Atoi(paddingSpecSplit[0])
	if err != nil {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	paddingY, err := strconv.Atoi(paddingSpecSplit[1])
	if err != nil {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	m.paddingX = paddingX
	m.paddingY = paddingY

	file, err := os.Open(m.fileName)
	if err != nil {
		return updatePreviewMsg{
			decodeError{fmt.Errorf("Error opening the file: %w", err)}, nil,
		}
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return updatePreviewMsg{
			decodeError{fmt.Errorf("Error reading the image: %w", err)}, nil,
		}
	}

	bounds := img.Bounds().Max
	imageWidth := bounds.X
	imageHeight := bounds.Y

	m.unpadded = imageWidth%(BRAILLE_WIDTH) == 0 &&
		imageHeight%(BRAILLE_HEIGHT) == 0 &&
		!isShaded(img.At(imageWidth, imageHeight))

	if !m.unpadded {
		if imageWidth%(BRAILLE_WIDTH+paddingX) != 0 {
			err = InvalidImgDimensionE{imageWidth, BRAILLE_WIDTH + paddingX, true}
			return updatePreviewMsg{err, nil}
		}

		if imageHeight%(BRAILLE_HEIGHT+paddingY) != 0 {
			err = InvalidImgDimensionE{imageHeight, BRAILLE_HEIGHT + paddingY, false}
			return updatePreviewMsg{err, nil}
		}
	}

	// TODO: Check if divisible checks both fail, combine if ever
	brailleWithPaddingW := (BRAILLE_WIDTH + paddingX)
	brailleWithPaddingH := (BRAILLE_HEIGHT + paddingY)

	if m.unpadded {
		brailleWithPaddingW = BRAILLE_WIDTH
		brailleWithPaddingH = BRAILLE_HEIGHT
	}

	brailleW := imageWidth / brailleWithPaddingW
	brailleH := imageHeight / brailleWithPaddingH

	pixels := make([][]rune, brailleH)
	for y := range pixels {
		pixels[y] = make([]rune, brailleW)
	}

	bitRep := make([]rune, 0, 8)
	for bigYOff := 0; bigYOff < imageHeight; bigYOff += brailleWithPaddingH {
		for bigXOff := 0; bigXOff < imageWidth; bigXOff += brailleWithPaddingW {
			for charYOff := BRAILLE_HEIGHT - 1; charYOff >= 0; charYOff -= 1 {
				for charXOff := BRAILLE_WIDTH - 1; charXOff >= 0; charXOff -= 1 {
					x := bigXOff + charXOff
					y := bigYOff + charYOff

					if isShaded(img.At(x, y)) {
						bitRep = append(bitRep, '1')
					} else {
						bitRep = append(bitRep, '0')
					}
				}
			}

			charX := bigXOff / brailleWithPaddingW
			charY := bigYOff / brailleWithPaddingH

			brailleIdx, _ := strconv.ParseUint(string(bitRep), 2, 8)
			pixels[charY][charX] = brailleLookup[brailleIdx]

			bitRep = bitRep[:0]
		}
	}

	return updatePreviewMsg{nil, pixels}
}

func (m *previewArtModel) modifyPaddingState(unpadded bool) error {
	rFile, err := os.Open(m.fileName)
	if err != nil {
		return decodeError{err}
	}

	oldImage, err := png.Decode(rFile)
	if err != nil {
		rFile.Close()
		return decodeError{err}
	}
	rFile.Close()

	bounds := oldImage.Bounds().Max
	oldImageWidth := bounds.X
	oldImageHeight := bounds.Y

	type bDimension struct{ w, h int }

	braillePaddedW := (BRAILLE_WIDTH + m.paddingX)
	braillePaddedH := (BRAILLE_HEIGHT + m.paddingY)

	beforeToggle := bDimension{braillePaddedW, braillePaddedH}
	afterToggle := bDimension{BRAILLE_WIDTH, BRAILLE_HEIGHT}

	if m.unpadded {
		beforeToggle, afterToggle = afterToggle, beforeToggle
	}

	if oldImageWidth%(beforeToggle.w) != 0 {
		return InvalidImgDimensionE{oldImageWidth, beforeToggle.w, true}
	}

	if oldImageHeight%(beforeToggle.h) != 0 {
		return InvalidImgDimensionE{oldImageHeight, beforeToggle.h, false}
	}

	if m.unpadded == unpadded {
		return nil
	}

	stats, err := os.Stat(m.fileName)
	if err != nil {
		return err
	}

	if time.Since(stats.ModTime()) < time.Second {
		return silentError{err}
	}

	charsX := oldImageWidth / beforeToggle.w
	charsY := oldImageHeight / beforeToggle.h

	wFile, err := os.Create(m.fileName)
	if err != nil {
		return decodeError{err}
	}

	newImage := image.NewNRGBA(image.Rect(0, 0, charsX*afterToggle.w, charsY*afterToggle.h))

	for charY := range charsY {
		for charX := range charsX {
			for brailleYOff := range BRAILLE_HEIGHT {
				for brailleXOff := range BRAILLE_WIDTH {
					beforeX := charX*beforeToggle.w + brailleXOff
					beforeY := charY*beforeToggle.h + brailleYOff

					afterX := charX*afterToggle.w + brailleXOff
					afterY := charY*afterToggle.h + brailleYOff

					pxBefore := oldImage.At(beforeX, beforeY)
					newImage.Set(afterX, afterY, pxBefore)
				}
			}
		}
	}

	if unpadded {
		err := png.Encode(wFile, newImage)
		return err
	}

	for charY := range charsY {
		for charX := range charsX {
			for paddingYOff := range m.paddingY {
				for brailleXOff := range BRAILLE_WIDTH {
					x := brailleXOff + charX*afterToggle.w
					y := BRAILLE_HEIGHT + paddingYOff + charY*afterToggle.h

					newImage.Set(x, y, color.Transparent)
				}
			}

			for paddingXOff := range m.paddingX {
				for brailleYOff := range BRAILLE_HEIGHT {
					x := BRAILLE_WIDTH + paddingXOff + charX*afterToggle.w
					y := brailleYOff + charY*afterToggle.h

					newImage.Set(x, y, color.Transparent)
				}
			}
		}
	}

	err = png.Encode(wFile, newImage)
	return err
}

func isShaded(c color.Color) bool {
	pxColor := color.NRGBAModel.Convert(c).(color.NRGBA)
	if int(pxColor.A) < alphaThreshold {
		return false
	}

	// 3 color channels * 2/3 brightness = 2 multiplier to alpha
	sumOfColors := int32(pxColor.R) + int32(pxColor.G) + int32(pxColor.B)
	return sumOfColors < 2*int32(pxColor.A)
}

func (m *previewArtModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case updatePreviewMsg:
		m.watchTicker = !m.watchTicker
		m.err = msg.err

		if _, shouldPanic := msg.err.(decodeError); shouldPanic {
			panicMsg := panicMsgModel(
				fmt.Sprintf("Filename: %v\n%v", m.fileName, msg.err),
			)
			return panicMsg, tea.Quit
		}

		if msg.err == nil {
			m.pixels = msg.pixels
		}

		return m.Tick()

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			//  TODO: resize operation
		case "t":
			if m.err != nil {
				return m, nil
			}

			m.err = m.modifyPaddingState(!m.unpadded)
			if m.err != nil {
				if _, isSilent := m.err.(silentError); isSilent {
					return m, nil
				}

				// TODO: Maybe create a panic message here?
				return panicMsgModel(m.err.Error()), nil
			}

			m.unpadded = !m.unpadded
			return m, nil
		}
	}

	return m, nil
}

var previewBorder = lipgloss.NewStyle().Border(lipgloss.InnerHalfBlockBorder())

func (m *previewArtModel) View() string {
	renderedPixels := func() string {
		if len(m.pixels) == 0 {
			return "xxxxx\nxxxxx\nxxxxx\nxxxxx\nxxxxx"
		}

		builder := strings.Builder{}
		builder.WriteString(string(m.pixels[0]))

		for _, line := range m.pixels[1:] {
			builder.WriteRune('\n')
			builder.WriteString(string(line))
		}
		return previewBorder.Render(builder.String())
	}()

	watchTickerView := "_ watching file /"
	if !m.watchTicker {
		watchTickerView = "\\ watching file _"
	}

	if m.err == nil {
		watchView := lipgloss.JoinVertical(lipgloss.Center, renderedPixels, watchTickerView, "")
		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			m.fileName,
			watchView,
			"(t to toggle padding)",
			fmt.Sprintf("unpadded?: %v, padding:(%v,%v)", m.unpadded, m.paddingX, m.paddingY),
		)
	}

	watchTickerView = "_ watching (invalid) file /"
	if !m.watchTicker {
		watchTickerView = "\\ watching (invalid) file _"
	}

	errorPrompt := fmt.Sprintf("Error processing the file:\n%v", m.err)
	watchView := lipgloss.JoinVertical(lipgloss.Center, renderedPixels, "", watchTickerView)

	return lipgloss.JoinVertical(lipgloss.Left, m.fileName, watchView, "", errorPrompt, "")
}
