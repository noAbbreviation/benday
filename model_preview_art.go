package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
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

	isUnpadded bool
}

func (err InvalidImgDimensionE) Error() string {
	measureName := "width"
	if !err.errorOnX {
		measureName = "height"
	}

	minusOneText := ""
	if err.isUnpadded {
		minusOneText = " - 1"
	}

	return fmt.Sprintf(
		"Invalid image dimension. Expected %v%v to be divisible by %v, but is instead %v px.",
		measureName,
		minusOneText,
		err.mustBeDivisibleBy,
		err.measure,
	)
}

type previewArtModel struct {
	fileName    string
	writeSignal chan struct{}

	processError    error
	updateViewError error

	paddingX int
	paddingY int
	pixels   [][]rune

	watchTicker bool
	unpadded    bool

	notifMessage string
	notifTime    time.Time

	_fromArgs  bool
	rOpts      resizeOptionStore
	exportOpts exportOptionStore
}

type resizeOptionStore struct {
	inputs         [2]int
	toResizeHeight bool

	resizing          bool
	showConfirmPrompt bool
}

type exportOptionStore struct {
	exporting         bool
	showConfirmPrompt bool

	input textinput.Model
}

type canvasMeasure struct {
	imageWidth  int
	imageHeight int
	isUnpadded  bool

	charsX int
	charsY int

	brailleW int
	brailleH int
}

func newPreviewArtModel(fileName string) *previewArtModel {
	textInput := textinput.New()
	textInput.Placeholder = ""
	textInput.CharLimit = 64
	textInput.Width = 64
	textInput.Prompt = ""
	textInput.Validate = isValidFileName

	newModel := &previewArtModel{
		fileName:    fileName,
		writeSignal: make(chan struct{}, 1),
		exportOpts: exportOptionStore{
			input: textInput,
		},
	}
	pixelData := newModel.GetPixels()

	newModel.pixels = pixelData.pixels
	newModel.updateViewError = pixelData.err

	return newModel
}

func previewArtModelFromArgs(fileName string) *previewArtModel {
	previewModel := newPreviewArtModel(fileName)
	previewModel._fromArgs = true

	return previewModel
}

func (m *previewArtModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		func() tea.Msg { return m.GetPixels() },
	)
}

func (m *previewArtModel) Tick() (*previewArtModel, tea.Cmd) {
	return m, tea.Every(time.Millisecond*500, func(t time.Time) tea.Msg {
		if len(m.writeSignal) != 0 {
			<-m.writeSignal
		}

		m.watchTicker = !m.watchTicker
		return m.GetPixels()
	})
}

type updatePreviewMsg struct {
	err    error
	pixels [][]rune
}

func (model *previewArtModel) GetPixels() updatePreviewMsg {
	file, err := os.Open(model.fileName)
	if err != nil {
		err := decodeError{FileDoesNotExistError}
		return updatePreviewMsg{err, nil}
	}

	defer file.Close()

	dotChars := strings.Count(model.fileName, ".")
	if dotChars < 3 {
		return updatePreviewMsg{InvalidFileNameError, nil}
	}

	fileNameInfo := strings.Split(model.fileName, ".")
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

	if isValidPadding(paddingSpecSplit[0]) != nil || isValidPadding(paddingSpecSplit[1]) != nil {
		err := fmt.Errorf("Padding is an invalid value: %w", NotAPositiveNumberError)
		return updatePreviewMsg{err, nil}
	}

	paddingX, _ := strconv.Atoi(paddingSpecSplit[0])
	paddingY, _ := strconv.Atoi(paddingSpecSplit[1])

	model.paddingX = paddingX
	model.paddingY = paddingY

	m, err := getCanvasMeasurement(model.fileName, paddingX, paddingY)
	if err != nil {
		return updatePreviewMsg{err, nil}
	}

	model.unpadded = m.isUnpadded

	img, err := png.Decode(file)
	if err != nil {
		return updatePreviewMsg{
			decodeError{fmt.Errorf("Error reading the image: %w", err)}, nil,
		}
	}

	pixels := make([][]rune, m.charsY)
	for y := range pixels {
		pixels[y] = make([]rune, m.charsX)
	}

	bitRep := make([]rune, 0, 8)
	for charY := range m.charsY {
		for charX := range m.charsX {
			for charYOff := BRAILLE_HEIGHT - 1; charYOff >= 0; charYOff -= 1 {
				for charXOff := BRAILLE_WIDTH - 1; charXOff >= 0; charXOff -= 1 {
					x := charX*m.brailleW + charXOff
					y := charY*m.brailleH + charYOff

					if shadeType(img.At(x, y)) == colorShaded {
						bitRep = append(bitRep, '1')
					} else {
						bitRep = append(bitRep, '0')
					}
				}
			}

			brailleIdx, _ := strconv.ParseUint(string(bitRep), 2, 8)
			pixels[charY][charX] = brailleLookup[brailleIdx]

			bitRep = bitRep[:0]
		}
	}

	return updatePreviewMsg{nil, pixels}
}

func togglePaddingState(fileName string, paddingX int, paddingY int) error {
	fileStats, err := os.Stat(fileName)
	if err != nil {
		return decodeError{FileDoesNotExistError}
	}

	if time.Since(fileStats.ModTime()) < time.Second {
		return silentError{err}
	}

	m, err := getCanvasMeasurement(fileName, paddingX, paddingY)
	if err != nil {
		return err
	}

	type bDimension struct{ w, h int }

	beforeMeasure := bDimension{m.brailleW, m.brailleH}
	afterMeasure := beforeMeasure

	if m.isUnpadded {
		afterMeasure.w += paddingX
		afterMeasure.h += paddingY
	} else {
		afterMeasure.w -= paddingX
		afterMeasure.h -= paddingY
	}

	rFile, err := os.Open(fileName)
	if err != nil {
		return decodeError{FileDoesNotExistError}
	}

	oldImage, err := png.Decode(rFile)
	rFile.Close()

	if err != nil {
		return decodeError{err}
	}

	newImageMeasure := bDimension{m.charsX * afterMeasure.w, m.charsY * afterMeasure.h}
	if !m.isUnpadded {
		newImageMeasure.w += 1
		newImageMeasure.h += 1
	}

	newImage := draw.Image(image.NewNRGBA(image.Rect(0, 0, newImageMeasure.w, newImageMeasure.h)))
	for charY := range m.charsY {
		for charX := range m.charsX {
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

	if m.isUnpadded {
		newImage = drawPadding(newImage, paddingX, paddingY)
	}

	wFile, err := os.Create(fileName)
	if err != nil {
		return decodeError{err}
	}

	encodeError := png.Encode(wFile, newImage)
	return encodeError
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

func cleanCanvas(fileName string, paddingX int, paddingY int, removeNonGrayscale bool) error {
	fileStats, err := os.Stat(fileName)
	if err != nil {
		return decodeError{FileDoesNotExistError}
	}

	if time.Since(fileStats.ModTime()) < time.Second {
		return silentError{err}
	}

	m, err := getCanvasMeasurement(fileName, paddingX, paddingY)
	if err != nil {
		return err
	}

	file, err := os.Open(fileName)
	if err != nil {
		return decodeError{FileDoesNotExistError}
	}

	img, err := png.Decode(file)
	file.Close()

	if err != nil {
		return decodeError{err}
	}

	newImage := draw.Image(image.NewNRGBA(image.Rect(0, 0, m.imageWidth, m.imageHeight)))
	draw.Draw(newImage, img.Bounds(), img, image.Point{}, draw.Src)

	defaultCanvasImg := newCanvasImage(m.imageWidth, m.imageHeight, paddingX, paddingY, m.isUnpadded)
	maskForDefault := image.NewAlpha16(img.Bounds())

	for bigOffsetX := 0; bigOffsetX < m.imageWidth; bigOffsetX += m.brailleW {
		for bigOffsetY := 0; bigOffsetY < m.imageHeight; bigOffsetY += m.brailleH {
			for charX := range BRAILLE_WIDTH {
				for charY := range BRAILLE_HEIGHT {
					x := bigOffsetX + charX
					y := bigOffsetY + charY

					shade := shadeType(newImage.At(x, y))

					if shade == colorShaded {
						colorBlack := color.NRGBA{0x33, 0x33, 0x33, 0xff}
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

	if m.isUnpadded {
		transparentImg := image.NewUniform(color.NRGBA{})

		verticalRect := image.Rect(m.imageWidth-1, 0, m.imageWidth, m.imageHeight)
		horizontalRect := image.Rect(0, m.imageHeight-1, m.imageWidth, m.imageHeight)

		draw.Draw(newImage, verticalRect, transparentImg, image.Point{}, draw.Src)
		draw.Draw(newImage, horizontalRect, transparentImg, image.Point{}, draw.Src)
	} else {
		newImage = drawPadding(newImage, paddingX, paddingY)
	}

	file, err = os.Create(fileName)
	if err != nil {
		return err
	}

	encodeError := png.Encode(file, newImage)
	return encodeError
}

func getCanvasMeasurement(fileName string, paddingX int, paddingY int) (canvasMeasure, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return canvasMeasure{}, decodeError{FileDoesNotExistError}
	}

	config, err := png.DecodeConfig(file)
	file.Close()

	if err != nil {
		return canvasMeasure{}, decodeError{err}
	}

	imageTestWidth := config.Width
	imageTestHeight := config.Height

	brailleW := BRAILLE_WIDTH + paddingX
	brailleH := BRAILLE_HEIGHT + paddingY

	padded := imageTestWidth%brailleW == 0 && imageTestHeight%brailleH == 0
	if !padded {
		brailleW = BRAILLE_WIDTH
		brailleH = BRAILLE_HEIGHT

		imageTestWidth -= 1
		imageTestHeight -= 1
	}

	charsX := imageTestWidth / brailleW
	charsY := imageTestHeight / brailleH

	if charsX*brailleW != imageTestWidth {
		err := InvalidImgDimensionE{config.Width, brailleW, true, !padded}
		return canvasMeasure{}, err
	}

	if charsY*brailleH != imageTestHeight {
		err := InvalidImgDimensionE{config.Height, brailleW, true, !padded}
		return canvasMeasure{}, err
	}

	measurements := canvasMeasure{
		imageWidth:  config.Width,
		imageHeight: config.Height,
		isUnpadded:  !padded,
		charsX:      charsX,
		charsY:      charsY,
		brailleW:    brailleW,
		brailleH:    brailleH,
	}
	return measurements, nil
}

func (m *previewArtModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.rOpts.resizing {
				m.rOpts.resizing = false
				return m, nil
			}

			if m.exportOpts.showConfirmPrompt {
				m.exportOpts.showConfirmPrompt = false
				m.processError = nil

				return m, nil
			}

			if m.exportOpts.exporting {
				m.exportOpts.exporting = false
				return m, nil
			}

			if m._fromArgs {
				return m, tea.Quit
			}

			startModel := newBendayStartModel()
			return startModel, startModel.Init()
		}
	}

	if len(m.writeSignal) != 0 {
		if _, isUpdateMsg := msg.(updatePreviewMsg); !isUpdateMsg {
			return m, nil
		}
	}

	if opts := &m.exportOpts; opts.exporting {
		if m.processError != nil {
			if _, ok := msg.(tea.KeyMsg); ok {
				if opts.showConfirmPrompt {
					opts.showConfirmPrompt = false
					m.processError = nil

					focusMsg := opts.input.Focus()
					return m, focusMsg
				}
			}

			if _, isUpdateMsg := msg.(updatePreviewMsg); !isUpdateMsg {
				return m, nil
			}
		}

		if m.processError == nil {
			if !opts.showConfirmPrompt {
				if msg, isKeyMsg := msg.(tea.KeyMsg); isKeyMsg {
					switch msg.String() {
					case "enter":
						opts.showConfirmPrompt = true
						return m, nil
					}
				}
			}

			if opts.showConfirmPrompt {
				switch msg := msg.(type) {
				case tea.KeyMsg:
					switch msg.String() {
					case "y", "enter":
						if err := exportBraille(opts.input.Value(), m.pixels); err != nil {
							m.processError = err
							return m, nil
						}

						m.notifTime = time.Now()
						m.notifMessage = "finished exporting to file!"

						opts.exporting = false
						opts.showConfirmPrompt = false

						return m, nil
					case "b":
						opts.showConfirmPrompt = false

						focusCmd := opts.input.Focus()
						return m, focusCmd
					}
				}
			}
		}

		if _, isUpdateMsg := msg.(updatePreviewMsg); !isUpdateMsg {
			var cmd tea.Cmd
			opts.input, cmd = opts.input.Update(msg)

			return m, cmd
		}
	}

	if opts := &m.rOpts; opts.resizing {
		toResizeIdx := 0
		if opts.toResizeHeight {
			toResizeIdx = 1
		}

		measure, err := getCanvasMeasurement(m.fileName, m.paddingX, m.paddingY)
		if err != nil {
			m.processError = err
			return m, nil
		}

		if msg, isKey := msg.(tea.KeyMsg); isKey {
			if m.processError != nil {
				return m, nil
			}

			switch msg.String() {
			case "+", ">", ".", "up":
				opts.inputs[toResizeIdx] += 1
			case "-", "<", ",", "down":
				opts.inputs[toResizeIdx] -= 1
			case "tab", "shift+tab", "left", "right", "ctrl+n", "ctrl+p":
				opts.toResizeHeight = !opts.toResizeHeight

			case "c":
				opts.resizing = false
				return m, nil

			case "enter":
				resizeX := opts.inputs[0]
				resizeY := opts.inputs[1]

				m.writeSignal <- struct{}{}
				m.processError = resizeCanvas(m.fileName, m.paddingX, m.paddingY, resizeX, resizeY)
				<-m.writeSignal

				if m.processError != nil {
					if _, isSilent := m.processError.(silentError); isSilent {
						m.processError = nil
						return m, nil
					}

					return panicMsgModel(m.processError.Error()), nil
				}

				if resizeX != 0 || resizeY != 0 {
					m.notifTime = time.Now()
					m.notifMessage = "finished resizing the canvas!"
				}

				opts.resizing = false
				return m, nil
			}
		}

		if resizeWidth := opts.inputs[0]; resizeWidth+measure.charsX <= 0 {
			opts.inputs[0] = -(measure.charsX - 1)
		}

		if resizeHeight := opts.inputs[1]; resizeHeight+measure.charsY <= 0 {
			opts.inputs[1] = -(measure.charsY - 1)
		}
	}

	switch msg := msg.(type) {
	case updatePreviewMsg:
		m.updateViewError = msg.err

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
		if m.rOpts.resizing {
			return m, nil
		}

		if m.exportOpts.exporting {
			return m, nil
		}

		switch msg.String() {
		case "r":
			m.rOpts = resizeOptionStore{resizing: true}
			return m, nil
		case "e":
			m.exportOpts.exporting = true
			m.exportOpts.input.SetValue("")

			focusCmd := m.exportOpts.input.Focus()
			return m, focusCmd
		case "c", "C":
			if m.processError != nil {
				return m, nil
			}

			removeNonGrayscaleColors := msg.String() == "C"

			m.writeSignal <- struct{}{}
			m.processError = cleanCanvas(m.fileName, m.paddingX, m.paddingY, removeNonGrayscaleColors)
			<-m.writeSignal

			if m.processError != nil {
				if _, isSilent := m.processError.(silentError); isSilent {
					m.processError = nil
					return m, nil
				}

				return panicMsgModel(m.processError.Error()), nil
			}

			m.notifTime = time.Now()
			m.notifMessage = "finished cleaning the canvas!"
			if removeNonGrayscaleColors {
				m.notifMessage = "finished CLEANING the canvas!"
			}

			return m, nil
		case "t":
			if m.processError != nil {
				return m, nil
			}

			m.writeSignal <- struct{}{}
			m.processError = togglePaddingState(m.fileName, m.paddingX, m.paddingY)
			<-m.writeSignal

			if m.processError != nil {
				if _, isSilent := m.processError.(silentError); isSilent {
					m.processError = nil
					return m, nil
				}

				return panicMsgModel(m.processError.Error()), nil
			}

			m.notifTime = time.Now()
			m.notifMessage = "finished toggling the padding!"

			return m, nil
		}
	}

	return m, nil
}

func resizeCanvas(fileName string, paddingX int, paddingY int, resizeX int, resizeY int) error {
	if resizeX == 0 && resizeY == 0 {
		return nil
	}

	fileStats, err := os.Stat(fileName)
	if err != nil {
		return decodeError{FileDoesNotExistError}
	}

	if time.Since(fileStats.ModTime()) < time.Second {
		return silentError{err}
	}

	m, err := getCanvasMeasurement(fileName, paddingX, paddingY)
	if err != nil {
		return err
	}

	file, err := os.Open(fileName)
	if err != nil {
		return decodeError{FileDoesNotExistError}
	}

	oldImage, err := png.Decode(file)
	file.Close()

	if err != nil {
		return decodeError{err}
	}

	newCharsX := m.charsX + resizeX
	newCharsY := m.charsY + resizeY

	newImageWidth := newCharsX * m.brailleW
	newImageHeight := newCharsY * m.brailleH

	if m.isUnpadded {
		newImageWidth += 1
		newImageHeight += 1
	}

	newImage := image.NewNRGBA(image.Rect(0, 0, newImageWidth, newImageHeight))
	if resizeX > 0 || resizeY > 0 {
		defaultCanvas := newCanvasImage(newImage.Bounds().Dx(), newImage.Bounds().Dy(), paddingX, paddingY, m.isUnpadded)
		draw.Draw(newImage, newImage.Bounds(), defaultCanvas, image.Point{}, draw.Src)
	}

	draw.Draw(
		newImage,
		image.Rect(0, 0, min(m.charsX, newCharsX)*m.brailleW, min(m.charsY, newCharsY)*m.brailleH),
		oldImage,
		image.Point{},
		draw.Src,
	)

	file, err = os.Create(fileName)
	if err != nil {
		return err
	}

	encodeError := png.Encode(file, newImage)
	return encodeError
}

func exportBraille(fileName string, pixels [][]rune) error {
	_, err := os.Stat(fileName)
	if err == nil {
		return fmt.Errorf("File already exists.")
	}

	builder := bytes.Buffer{}
	for _, pixel := range pixels[0] {
		builder.WriteRune(pixel)
	}

	for _, line := range pixels[1:] {
		builder.WriteRune('\n')
		for _, pixel := range line {
			builder.WriteRune(pixel)
		}
	}

	err = os.WriteFile(fileName, builder.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Error writing to the file: %v", err)
	}

	return nil
}

var (
	previewBorder      = lipgloss.NewStyle().Border(lipgloss.InnerHalfBlockBorder())
	whiteSpaceWithX    = lipgloss.WithWhitespaceChars("x")
	whiteSpaceWithPlus = lipgloss.WithWhitespaceChars("+")

	erroredCanvas = previewBorder.Render("xxxxx\nxxxxx\nxxxxx\nxxxxx\nxxxxx")
)

func (m *previewArtModel) View() string {
	renderedPixels := func() string {
		if len(m.pixels) == 0 {
			return erroredCanvas
		}

		if !m.rOpts.resizing {
			builder := strings.Builder{}
			for _, pixel := range m.pixels[0] {
				builder.WriteRune(pixel)
			}

			for _, line := range m.pixels[1:] {
				builder.WriteRune('\n')
				for _, pixel := range line {
					builder.WriteRune(pixel)
				}
			}

			return previewBorder.Render(builder.String())
		}

		measure, err := getCanvasMeasurement(m.fileName, m.paddingX, m.paddingY)
		if err != nil {
			return erroredCanvas
		}

		newCharsX := m.rOpts.inputs[0] + measure.charsX
		newCharsY := m.rOpts.inputs[1] + measure.charsY

		renderedDimensionX := min(newCharsX, measure.charsX)
		renderedDimensionY := min(newCharsY, measure.charsY)

		whiteSpaceStyleX := whiteSpaceWithPlus
		whiteSpaceStyleY := whiteSpaceWithPlus

		if newCharsX < measure.charsX {
			whiteSpaceStyleX = whiteSpaceWithX
		}

		if newCharsY < measure.charsY {
			whiteSpaceStyleY = whiteSpaceWithX
		}

		builder := strings.Builder{}
		for _, pixel := range m.pixels[0][:renderedDimensionX] {
			builder.WriteRune(pixel)
		}

		for _, line := range m.pixels[1:renderedDimensionY] {
			builder.WriteRune('\n')
			for _, pixel := range line[:renderedDimensionX] {
				builder.WriteRune(pixel)
			}
		}

		renderedCanvas := builder.String()
		if newCharsX > measure.charsX {
			renderedCanvas = lipgloss.PlaceHorizontal(
				max(newCharsX, measure.charsX),
				lipgloss.Left,
				renderedCanvas,
				whiteSpaceStyleX,
			)
			renderedCanvas = lipgloss.PlaceVertical(
				max(newCharsY, measure.charsY),
				lipgloss.Top,
				renderedCanvas,
				whiteSpaceStyleY,
			)
		} else {
			renderedCanvas = lipgloss.PlaceVertical(
				max(newCharsY, measure.charsY),
				lipgloss.Top,
				renderedCanvas,
				whiteSpaceStyleY,
			)
			renderedCanvas = lipgloss.PlaceHorizontal(
				max(newCharsX, measure.charsX),
				lipgloss.Left,
				renderedCanvas,
				whiteSpaceStyleX,
			)
		}

		borderedCanvas := previewBorder.Render(renderedCanvas)
		if m.rOpts.toResizeHeight {
			return lipgloss.JoinVertical(lipgloss.Center, borderedCanvas, " # \n###")
		} else {
			return lipgloss.JoinHorizontal(lipgloss.Center, borderedCanvas, " #\n##\n #")
		}
	}()

	watchTickerView := "_ watching file /"
	if !m.watchTicker {
		watchTickerView = "\\ watching file _"
	}

	if opts := m.exportOpts; opts.exporting {
		if m.processError != nil {
			return lipgloss.JoinVertical(
				lipgloss.Left,
				"",
				fmt.Sprintf("Viewing %v", m.fileName),
				renderedPixels,
				watchTickerView,
				"",
				"Exporting braille characters to file:",
				"",
				"  Error creating the file:",
				fmt.Sprintf("  %v", m.processError.Error()),
				"",
				"(export failed) (any key to go back)",
				"",
			)
		}

		if opts.showConfirmPrompt {
			return lipgloss.JoinVertical(
				lipgloss.Left,
				"",
				fmt.Sprintf("Viewing %v", m.fileName),
				renderedPixels,
				watchTickerView,
				"",
				"Exporting braille characters to file:",
				"",
				"  Are you sure you want to create this file?",
				fmt.Sprintf("  \"%v\"", opts.input.Value()),
				"",
				"(exporting) (y/enter to confirm, b/esc to go back)",
				"",
			)
		}

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			fmt.Sprintf("Viewing %v", m.fileName),
			renderedPixels,
			watchTickerView,
			"",
			"Exporting braille characters to file:",
			fmt.Sprintf("File name: %v", opts.input.View()),
			"",
			"(exporting) (enter to continue, ctrl-c to exit program, esc to go back)",
			"",
		)
	}

	if m.updateViewError == nil {
		notifMessage := ""
		if notifTime := m.notifTime; !notifTime.IsZero() && time.Since(notifTime) < time.Millisecond*2_500 {
			notifMessage = ", " + m.notifMessage
		}

		tooltipText := "(t to toggle padding, c/C to clean canvas, r to resize canvas, e to export, ctrl-c to exit, esc to go back)"
		if opts := m.rOpts; opts.resizing {
			tooltipText = "(resizing) (+/- to adjust canvas, tab to change direction, c to cancel, enter to confirm, esc to go back)"
		}

		return lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			fmt.Sprintf("Viewing %v", m.fileName),
			renderedPixels,
			watchTickerView,
			"",
			tooltipText,
			fmt.Sprintf("padded?: %v%v", !m.unpadded, notifMessage),
		)
	}

	watchTickerView = "_ watching (invalid) file /"
	if !m.watchTicker {
		watchTickerView = "\\ watching (invalid) file _"
	}

	errorPrompt := fmt.Sprintf("Error processing the image:\n%v", m.updateViewError)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("Viewing %v", m.fileName),
		renderedPixels,
		"",
		watchTickerView,
		"",
		errorPrompt,
		"",
	)
}
