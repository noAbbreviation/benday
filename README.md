# Benday

![Watching a canvas in benday](./docs/benday_preview_art.gif)

Workflow CLI for editing braille ASCII art.
You can edit an image on your editor of choice, and it will show to the terminal on what it will look like.

## Features

#### Creating benday files

![Creating a file in benday](./docs/benday_create_files.gif)

> [!NOTE]
> Benday files ends with `.by.png`

#### Near real time feedback when saving the canvas

![Watching a canvas in benday](./docs/benday_preview_art.gif)

#### Ignores non black colored pixels for comments

![Benday ignoring non-black pixels](./docs/benday_ignores_non_black.gif)

---

### Commands for ease of use

![Cleaning the canvas in benday](./docs/benday_clean_canvas.gif)

Pressing c will clean the canvas (not the comment pixels)

---

![Cleaning the canvas including comments in benday](./docs/benday_cleaning_comments.gif)

Pressing **<u>C</u>** will clean the canvas (including the comment pixels)

---

![Toggling benday file padding to move pixels around](./docs/benday_toggle_padding.gif)

Pressing t will toggle the canvas' padding. You can move stuff more freely now.

---

![Resizing the benday canvas](./docs/benday_resize_canvas.gif)

Pressing r will bring up the resize interface where you can resize the canvas.

---

#### Export/import of braille ascii art

![Exporting benday png to braille ascii](./docs/benday_exporting_canvas.gif)

![Import braille ascii art to benday](./docs/benday_importing_braille_ascii.gif)

## Installation

If you have at least Go 1.23, install using the following command:

```
go install github.com/noAbbreviation/benday
```

> [!NOTE]
> A release may be in the works.

## Future stuff (maybe)

- Resizable and movable canvas viewport for bigger canvases
- OS native file locking
