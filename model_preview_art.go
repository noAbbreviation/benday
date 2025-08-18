package main

import (
	"errors"
	"fmt"
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
	InvalidFileNameError = errors.New("Invalid file name. File must end in the form \"*.<pX>x<pY>.by.png\".")

	alphaThreshold = 0xff / 3
)

var ()

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

	pixels   [][]rune
	isPadded bool

	brailleWidthC  int
	brailleHeightC int
	paddingX       int
	paddingY       int

	rOpts resizeOptionStore
}

type resizeOptionStore struct {
	resizing          bool
	showConfirmPrompt bool
	inputs            *[2]textinput.Model
}

func newPreviewArtModel(fileName string) (*previewArtModel, error) {
	newModel := &previewArtModel{fileName: fileName}
	if msg := newModel.GetPixels(); msg.err != nil {
		return nil, msg.err
	}

	return newModel, nil
}

func (m *previewArtModel) Init() tea.Cmd {
	return m.Tick()
}

func (m *previewArtModel) Tick() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return m.GetPixels()
	})
}

type updateMsg struct {
	err    error
	pixels [][]rune
}

// TODO: Clean options: restore unshaded, non-comment pixels

func (m *previewArtModel) GetPixels() updateMsg {
	dotChars := strings.Count(m.fileName, ".")
	if dotChars < 3 {
		return updateMsg{InvalidFileNameError, nil}
	}

	//  TODO: check for file's modTime() to be less redundant

	fileNameInfo := strings.Split(m.fileName, ".")
	slices.Reverse(fileNameInfo)

	if imgExtension := fileNameInfo[0]; imgExtension != "png" {
		return updateMsg{InvalidFileNameError, nil}
	}

	if hasBy := fileNameInfo[1] == "by"; !hasBy {
		return updateMsg{InvalidFileNameError, nil}
	}

	paddingSpec := fileNameInfo[2]
	if strings.Count(paddingSpec, "x") != 1 {
		return updateMsg{InvalidFileNameError, nil}
	}

	paddingSpecSplit := strings.Split(paddingSpec, "x")

	paddingX, err := strconv.Atoi(paddingSpecSplit[0])
	if err != nil {
		return updateMsg{InvalidFileNameError, nil}
	}

	paddingY, err := strconv.Atoi(paddingSpecSplit[1])
	if err != nil {
		return updateMsg{InvalidFileNameError, nil}
	}

	file, err := os.Open(m.fileName)
	if err != nil {
		return updateMsg{fmt.Errorf("Error opening the file: %v", err), nil}
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return updateMsg{fmt.Errorf("Error reading the image: %v", err), nil}
	}

	bounds := img.Bounds().Max
	imageWidth := bounds.X
	imageHeight := bounds.Y

	if imageWidth%(BRAILLE_WIDTH+paddingX) != 0 {
		err = InvalidImgDimensionE{imageWidth, BRAILLE_WIDTH + paddingX, true}
		return updateMsg{err, nil}
	}

	if imageHeight%(BRAILLE_HEIGHT+paddingY) != 0 {
		err = InvalidImgDimensionE{imageHeight, BRAILLE_HEIGHT + paddingY, false}
		return updateMsg{err, nil}
	}

	brailleW := imageWidth / (BRAILLE_WIDTH + paddingX)
	brailleH := imageHeight / (BRAILLE_WIDTH + paddingY)

	pixels := make([][]rune, brailleH)
	for y := range pixels {
		pixels[y] = make([]rune, brailleW)
	}

	bitRep := make([]rune, 0, 8)
	for bigYOff := 0; bigYOff < imageHeight; bigYOff += paddingY + BRAILLE_HEIGHT {
		for bigXOff := 0; bigXOff < imageWidth; bigXOff += paddingX + BRAILLE_WIDTH {
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

			brailleIdx, err := strconv.ParseUint(string(bitRep), 2, 8)
			if err != nil {
				return updateMsg{err, nil}
			}

			charX := bigXOff / (BRAILLE_WIDTH + paddingX)
			charY := bigYOff / (BRAILLE_HEIGHT + paddingY)
			pixels[charY][charX] = brailleLookup[brailleIdx]

			bitRep = bitRep[:0]
		}
	}

	return updateMsg{nil, pixels}
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
	case updateMsg:
		if err := msg.err; err != nil {
			//  TODO: Store and show error
			fmt.Printf("Error occured on parsing file: %v\n", err)
			os.Exit(1)
		}

		m.pixels = msg.pixels
		return m, m.Tick()
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			//  TODO: resize operation
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
		return lipgloss.JoinVertical(lipgloss.Center, renderedPixels, watchTickerView, "")
	}

	return bordered.Render(builder.String()) + "\n"
}
