package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
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

// TODO: Block updates when file operation is in place (using atomic.Bool)
type previewArtModel struct {
	fileName  string
	fileWrite chan struct{}
	err       error

	paddingX int
	paddingY int
	pixels   [][]rune

	watchTicker bool
	unpadded    bool

	notifMessage string
	notifTime    time.Time

	rOpts resizeOptionStore
}

type resizeOptionStore struct {
	resizing          bool
	showConfirmPrompt bool
	inputs            *[2]textinput.Model
}

func newPreviewArtModel(fileName string) *previewArtModel {
	newModel := &previewArtModel{
		fileName:  fileName,
		fileWrite: make(chan struct{}, 1),
	}
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
	return m, tea.Every(time.Millisecond*500, func(t time.Time) tea.Msg {
		return m.GetPixels()
	})
}

type updatePreviewMsg struct {
	err    error
	pixels [][]rune
}

func (m *previewArtModel) GetPixels() updatePreviewMsg {
	dotChars := strings.Count(m.fileName, ".")
	if dotChars < 3 {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	fileNameInfo := strings.Split(m.fileName, ".")
	{
		start := 0
		end := len(fileNameInfo) - 1

		for start < end {
			fileNameInfo[start], fileNameInfo[end] = fileNameInfo[end], fileNameInfo[start]

			start += 1
			end -= 1
		}
	}

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

	img, err := png.Decode(file)
	file.Close()

	if err != nil {
		return updatePreviewMsg{
			decodeError{fmt.Errorf("Error reading the image: %w", err)}, nil,
		}
	}

	bounds := img.Bounds().Max
	imageWidth := bounds.X
	imageHeight := bounds.Y

	m.unpadded = imageWidth%(BRAILLE_WIDTH) == 0 &&
		imageHeight%(BRAILLE_HEIGHT) == 0

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

					if shadeType(img.At(x, y)) == colorShaded {
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

func togglePaddingState(fileName string, currentlyUnpadded bool, paddingX int, paddingY int) (error, bool) {
	fileStats, err := os.Stat(fileName)
	if err != nil {
		return decodeError{err}, currentlyUnpadded
	}

	if time.Since(fileStats.ModTime()) < time.Second {
		return silentError{err}, currentlyUnpadded
	}

	rFile, err := os.Open(fileName)
	if err != nil {
		return decodeError{err}, currentlyUnpadded
	}

	oldImage, err := png.Decode(rFile)
	rFile.Close()

	if err != nil {
		return decodeError{err}, currentlyUnpadded
	}

	bounds := oldImage.Bounds().Max
	oldImageWidth := bounds.X
	oldImageHeight := bounds.Y

	type bDimension struct{ w, h int }

	braillePaddedW := BRAILLE_WIDTH + paddingX
	braillePaddedH := BRAILLE_HEIGHT + paddingY

	beforeMeasure := bDimension{braillePaddedW, braillePaddedH}
	afterMeasure := bDimension{BRAILLE_WIDTH, BRAILLE_HEIGHT}

	if currentlyUnpadded {
		beforeMeasure, afterMeasure = afterMeasure, beforeMeasure
	}

	if oldImageWidth%(beforeMeasure.w) != 0 {
		return InvalidImgDimensionE{oldImageWidth, beforeMeasure.w, true}, currentlyUnpadded
	}

	if oldImageHeight%(beforeMeasure.h) != 0 {
		return InvalidImgDimensionE{oldImageHeight, beforeMeasure.h, false}, currentlyUnpadded
	}

	charsX := oldImageWidth / beforeMeasure.w
	charsY := oldImageHeight / beforeMeasure.h

	newImage := draw.Image(image.NewNRGBA(image.Rect(0, 0, charsX*afterMeasure.w, charsY*afterMeasure.h)))

	for charY := range charsY {
		for charX := range charsX {
			for brailleYOff := range BRAILLE_HEIGHT {
				for brailleXOff := range BRAILLE_WIDTH {
					beforeX := charX*beforeMeasure.w + brailleXOff
					beforeY := charY*beforeMeasure.h + brailleYOff

					afterX := charX*afterMeasure.w + brailleXOff
					afterY := charY*afterMeasure.h + brailleYOff

					pxBefore := oldImage.At(beforeX, beforeY)
					newImage.Set(afterX, afterY, pxBefore)
				}
			}
		}
	}

	if currentlyUnpadded {
		newImage = drawPadding(newImage, paddingX, paddingY)
	}

	wFile, err := os.Create(fileName)
	if err != nil {
		return decodeError{err}, currentlyUnpadded
	}

	encodeError := png.Encode(wFile, newImage)
	return encodeError, !currentlyUnpadded
}

type shadedType int

const (
	colorTransparent shadedType = iota
	colorNonGrayscale
	colorNonShaded
	colorShaded
)

// This ignores sufficiently translucent, non-grayscale, and light colors.
func shadeType(c color.Color) shadedType {
	pxColor := color.NRGBAModel.Convert(c).(color.NRGBA)
	r, g, b, a := uint32(pxColor.R), uint32(pxColor.G), uint32(pxColor.B), uint32(pxColor.A)

	if 3*a < 0xff {
		return colorTransparent
	}

	// Derivation of "deviation":
	// deviation = (abs(r, g) + abs(g, b) + abs(r, b)) / 3
	// deviation = (r-g + g-b + r-b) / 3       (without loss of generality: r >= g >= b)
	// deviation = 2 * (r-b) / 3
	// deviation = 2 * (maximum(r, g, b) - minimum(r, g, b)) / 3
	// (then multiplied the divisor to the other side)

	// Originally as:
	// `if deviation := (abs(r - g) + abs(g - b) + abs(r - b)) / 3; deviation > 0xff/16 { ... }`
	if deviation := 2 * (max(r, g, b) - min(r, g, b)); 16*deviation > 3*0xff {
		return colorNonGrayscale
	}

	// 3 color channels * 2/3 brightness = 2 multiplier to alpha
	sumOfColors := r + g + b
	if sumOfColors < 2*a {
		return colorShaded
	} else {
		return colorNonShaded
	}
}

func cleanCanvas(fileName string, isUnpadded bool, paddingX int, paddingY int, removeNonGrayscale bool) error {
	fileStats, err := os.Stat(fileName)
	if err != nil {
		return decodeError{err}
	}

	if time.Since(fileStats.ModTime()) < time.Second {
		return silentError{err}
	}

	file, err := os.Open(fileName)
	if err != nil {
		return decodeError{err}
	}

	img, err := png.Decode(file)
	file.Close()

	if err != nil {
		return decodeError{err}
	}

	bounds := img.Bounds().Max
	imageWidth := bounds.X
	imageHeight := bounds.Y

	braillePaddedW := BRAILLE_WIDTH + paddingX
	braillePaddedH := BRAILLE_HEIGHT + paddingY

	if isUnpadded {
		braillePaddedW = BRAILLE_WIDTH
		braillePaddedH = BRAILLE_HEIGHT
	}

	newImage := draw.Image(image.NewNRGBA(img.Bounds()))
	draw.Draw(newImage, img.Bounds(), img, image.Point{}, draw.Src)

	defaultCanvasImg := newCanvasImage(imageWidth, imageHeight, paddingX, paddingY, isUnpadded)
	maskForDefault := image.NewAlpha16(img.Bounds())

	for bigOffsetX := 0; bigOffsetX < imageWidth; bigOffsetX += braillePaddedW {
		for bigOffsetY := 0; bigOffsetY < imageHeight; bigOffsetY += braillePaddedH {
			for charX := range BRAILLE_WIDTH {
				for charY := range BRAILLE_HEIGHT {
					x := bigOffsetX + charX
					y := bigOffsetY + charY

					shade := shadeType(newImage.At(x, y))

					if shade == colorShaded {
						colorBlack := color.NRGBA{0, 0, 0, 0xff}
						newImage.Set(x, y, colorBlack)

						continue
					}

					if removeNonGrayscale {
						maskForDefault.Set(x, y, color.Opaque)
						continue
					}

					if shade != colorNonGrayscale {
						maskForDefault.Set(x, y, color.Opaque)
					}
				}
			}
		}
	}

	draw.DrawMask(newImage, img.Bounds(), defaultCanvasImg, image.Point{}, maskForDefault, image.Point{}, draw.Over)

	if !isUnpadded {
		newImage = drawPadding(newImage, paddingX, paddingY)
	}

	file, err = os.Create(fileName)
	if err != nil {
		return err
	}

	encodeError := png.Encode(file, newImage)
	return encodeError
}

func (m *previewArtModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	if len(m.fileWrite) != 0 {
		<-m.fileWrite
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
		case "c", "C":
			if m.err != nil {
				return m, nil
			}

			removeNonGrayscaleColors := msg.String() == "C"

			m.fileWrite <- struct{}{}
			m.err = cleanCanvas(m.fileName, m.unpadded, m.paddingX, m.paddingY, removeNonGrayscaleColors)
			<-m.fileWrite

			if m.err != nil {
				if _, isSilent := m.err.(silentError); isSilent {
					m.err = nil
					return m, nil
				}

				return panicMsgModel(m.err.Error()), nil
			}

			m.notifTime = time.Now()
			m.notifMessage = "finished cleaning the canvas!"

			return m, nil
		case "t":
			if m.err != nil {
				return m, nil
			}

			m.fileWrite <- struct{}{}
			m.err, m.unpadded = togglePaddingState(m.fileName, m.unpadded, m.paddingX, m.paddingY)
			<-m.fileWrite

			if m.err != nil {
				if _, isSilent := m.err.(silentError); isSilent {
					m.err = nil
					return m, nil
				}

				return panicMsgModel(m.err.Error()), nil
			}

			m.notifTime = time.Now()
			m.notifMessage = "finished toggling the padding!"

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

		notifMessage := ""
		if notifTime := m.notifTime; !notifTime.IsZero() && time.Since(notifTime) < time.Millisecond*2_500 {
			notifMessage = ", " + m.notifMessage
		}

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			m.fileName,
			watchView,
			"(t to toggle padding, c/C to clean canvas)",
			fmt.Sprintf("unpadded?: %v%v", m.unpadded, notifMessage),
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
