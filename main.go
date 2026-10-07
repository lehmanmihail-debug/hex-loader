package main

import (
	"bytes"
	"fmt"
	"image/color"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// BoardInfo хранит информацию о подключенной плате
type BoardInfo struct {
	Port string
	FQBN string
}

// myTheme — кастомная тема с увеличенным размером и убранной тенью
type myTheme struct {
	fyne.Theme
}

func (t myTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 16
	case theme.SizeNamePadding:
		return 14
	default:
		return t.Theme.Size(name)
	}
}

func (t myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF}
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0xE0, G: 0xE0, B: 0xE0, A: 0xFF}
	case theme.ColorNameShadow:
		return color.Transparent
	default:
		return t.Theme.Color(name, variant)
	}
}

// buttonWithBorder — кастомная кнопка с обводкой
type buttonWithBorder struct {
	widget.Button
}

func newButtonWithBorder(label string, onTap func()) *buttonWithBorder {
	btn := &buttonWithBorder{}
	btn.ExtendBaseWidget(btn)
	btn.Text = label
	btn.OnTapped = onTap
	return btn
}

func (b *buttonWithBorder) CreateRenderer() fyne.WidgetRenderer {
	return &buttonRenderer{btn: b}
}

// buttonRenderer — рендерер кнопки с обводкой
type buttonRenderer struct {
	btn       *buttonWithBorder
	label     *canvas.Text
	bg        *canvas.Rectangle
	border    *canvas.Rectangle
	objects   []fyne.CanvasObject
	container *fyne.Container
}

func (r *buttonRenderer) init() {
	if r.container != nil {
		return
	}
	r.label = canvas.NewText(r.btn.Text, theme.Color(theme.ColorNameForeground))
	r.label.Alignment = fyne.TextAlignCenter
	r.label.TextSize = theme.TextSize()

	r.bg = canvas.NewRectangle(theme.Color(theme.ColorNameButton))
	r.bg.CornerRadius = 8

	r.border = canvas.NewRectangle(color.Transparent)
	r.border.StrokeWidth = 2
	r.border.StrokeColor = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	r.border.CornerRadius = 8

	r.container = container.NewStack(r.bg, r.border, r.label)
	r.objects = []fyne.CanvasObject{r.container}
}

func (r *buttonRenderer) Layout(size fyne.Size) {
	r.init()
	r.container.Resize(size)
}

func (r *buttonRenderer) MinSize() fyne.Size {
	r.init()
	minSize := r.container.MinSize()
	return fyne.NewSize(minSize.Width+40, minSize.Height+16)
}

func (r *buttonRenderer) Refresh() {
	r.init()
	r.label.Text = r.btn.Text
	r.label.Color = theme.Color(theme.ColorNameForeground)
	r.label.TextSize = theme.TextSize()

	if r.btn.Disabled() {
		r.bg.FillColor = theme.Color(theme.ColorNameDisabled)
		r.border.StrokeColor = color.NRGBA{R: 0x66, G: 0x66, B: 0x66, A: 0x80}
	} else {
		r.bg.FillColor = theme.Color(theme.ColorNameButton)
		r.border.StrokeColor = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	}
	r.bg.CornerRadius = 8
	r.border.CornerRadius = 8

	r.label.Refresh()
	r.bg.Refresh()
	r.border.Refresh()
	canvas.Refresh(r.btn)
}

func (r *buttonRenderer) Objects() []fyne.CanvasObject {
	r.init()
	return r.objects
}

func (r *buttonRenderer) Destroy() {}

// showCustomInformation — кастомный информационный диалог с кнопкой с обводкой
func showCustomInformation(title, message string, w fyne.Window) {
	label := widget.NewLabel(message)
	label.Wrapping = fyne.TextWrapWord

	var d *dialog.CustomDialog

	d = dialog.NewCustom(title, "", container.NewVBox(
		label,
		container.NewCenter(newButtonWithBorder("OK", func() {
			if d != nil {
				d.Hide()
			}
		})),
	), w)
	d.Resize(fyne.NewSize(400, 200))
	d.Show()
}

// checkArduinoCLI проверяет, доступен ли arduino-cli в PATH
func checkArduinoCLI() error {
	_, err := exec.LookPath("arduino-cli")
	return err
}

// checkUserInGroup проверяет, состоит ли текущий пользователь в указанной группе
func checkUserInGroup(groupName string) (bool, error) {
	currentUser, err := user.Current()
	if err != nil {
		return false, err
	}

	cmd := exec.Command("groups", currentUser.Username)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	groups := strings.Fields(string(output))
	for _, g := range groups {
		if g == groupName {
			return true, nil
		}
	}
	return false, nil
}

// showPermissionWarning показывает предупреждение о правах
func showPermissionWarning(a fyne.App, w fyne.Window) {
	var d *dialog.CustomDialog

	label := widget.NewLabel(
		"⚠️ У вас нет прав для работы с последовательными портами.\n\n" +
			"Чтобы Hex Loader мог определять и прошивать платы,\n" +
			"добавьте пользователя в группу dialout:\n\n" +
			"  sudo usermod -a -G dialout $USER\n\n" +
			"После этого выйдите из системы и зайдите заново.\n\n" +
			"Вы всё равно можете использовать программу,\n" +
			"но порты могут не определяться.",
	)

	btnOk := newButtonWithBorder("Понятно", func() {
		if d != nil {
			d.Hide()
		}
	})

	content := container.NewVBox(label, btnOk)
	d = dialog.NewCustom("Внимание", "", content, w)
	d.Resize(fyne.NewSize(500, 300))
	d.Show()
}

// uploadWithProgress выполняет загрузку с отображением статуса
func uploadWithProgress(hexPath, portPath, fqbn string, statusLabel *widget.Label) error {
	result := make(chan error)

	go func() {
		fyne.Do(func() {
			statusLabel.SetText("⏳ Подождите, идёт загрузка...")
			statusLabel.Refresh()
		})

		time.Sleep(50 * time.Millisecond)

		cmd := exec.Command(
			"arduino-cli",
			"upload",
			"-p", portPath,
			"--fqbn", fqbn,
			"--input-file", hexPath,
		)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			result <- fmt.Errorf("ошибка загрузки: %v: %s", err, stderr.String())
			return
		}
		result <- nil
	}()

	err := <-result

	if err != nil {
		fyne.Do(func() {
			statusLabel.SetText("❌ Ошибка загрузки")
			statusLabel.Refresh()
		})
		return err
	}

	fyne.Do(func() {
		statusLabel.SetText("✅ Загрузка успешно завершена!")
		statusLabel.Refresh()
	})
	return nil
}

func main() {
	a := app.NewWithID("com.example.hexloader")
	a.Settings().SetTheme(&myTheme{theme.DefaultTheme()})

	w := a.NewWindow("Загрузчик HEX в Arduino")
	w.Resize(fyne.NewSize(700, 480))

	var hexPath string
	var portPath string
	var fqbn string

	// Получаем preferences для сохранения настроек
	prefs := a.Preferences()

	// Загружаем последний выбранный HEX файл из настроек
	lastHexPath := prefs.String("lastHexPath")
	if lastHexPath != "" {
		hexPath = lastHexPath
	}

	hexLabel := widget.NewLabel("Файл не выбран")
	if hexPath != "" {
		hexLabel.SetText(hexPath)
	}

	portLabel := widget.NewLabel("Порт не выбран")
	statusLabel := widget.NewLabel("Готов к работе")

	// --- ИКОНКА В ИНТЕРФЕЙСЕ ---
	iconImage := canvas.NewImageFromFile("icon.png")
	iconImage.FillMode = canvas.ImageFillContain
	iconImage.SetMinSize(fyne.NewSize(64, 64))
	iconImage.Resize(fyne.NewSize(64, 64))
	iconImage.Refresh()

	// Создаём заголовок
	title := canvas.NewText("Загрузчик HEX файлов в Arduino", color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF})
	title.TextSize = 22
	title.TextStyle.Bold = true
	title.Alignment = fyne.TextAlignCenter

	// Собираем заголовок с иконкой
	header := container.NewVBox(
		container.NewCenter(iconImage),
		title,
	)

	// Кнопка выбора HEX с фильтром и сохранением пути
	btnSelectHex := newButtonWithBorder("📂 Выбрать HEX", func() {
		fileFilter := storage.NewExtensionFileFilter([]string{".hex"})
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			hexPath = reader.URI().Path()
			hexLabel.SetText(hexPath)
			statusLabel.SetText("HEX файл выбран")

			// Сохраняем путь в preferences
			prefs.SetString("lastHexPath", hexPath)
		}, w)
		fileDialog.SetFilter(fileFilter)
		fileDialog.Show()
	})

	btnSelectPort := newButtonWithBorder("🔌 Выбрать порт", func() {
		boards, err := getAvailableBoards()
		if err != nil {
			dialog.ShowError(fmt.Errorf("не удалось получить список плат: %v", err), w)
			return
		}
		if len(boards) == 0 {
			// Используем кастомный диалог вместо dialog.ShowInformation
			showCustomInformation("Нет плат", "Подключенные Arduino платы не найдены.", w)
			return
		}

		if len(boards) == 1 {
			selectBoard(boards[0], &portPath, &fqbn, portLabel, statusLabel, w)
			return
		}

		items := make([]string, len(boards))
		for i, b := range boards {
			label := b.Port
			if b.FQBN != "" && strings.Count(b.FQBN, ":") >= 2 {
				label += " (" + b.FQBN + ")"
			} else {
				label += " (тип не определён)"
			}
			items[i] = label
		}
		selected := widget.NewSelect(items, func(s string) {
			port := strings.Split(s, " ")[0]
			for _, b := range boards {
				if b.Port == port {
					selectBoard(b, &portPath, &fqbn, portLabel, statusLabel, w)
					break
				}
			}
		})
		dialog.ShowCustom("Выберите плату", "OK", selected, w)
	})

	btnUpload := newButtonWithBorder("⬆️ Загрузить", func() {
		if hexPath == "" {
			showCustomInformation("Ошибка", "Сначала выберите HEX файл.", w)
			return
		}
		if portPath == "" {
			showCustomInformation("Ошибка", "Сначала выберите порт.", w)
			return
		}
		if fqbn == "" {
			showFQBNInputDialog(w, &fqbn, statusLabel)
			if fqbn == "" {
				return
			}
		}

		// Проверяем, установлено ли ядро
		cmd := exec.Command("arduino-cli", "core", "list")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Run()
		coreName := strings.Split(fqbn, ":")[0] + ":" + strings.Split(fqbn, ":")[1]
		if !strings.Contains(out.String(), coreName) {
			dialog.ShowError(fmt.Errorf("ядро %s не установлено. Установите его через arduino-cli или скопируйте папку packages", coreName), w)
			return
		}

		go func() {
			err := uploadWithProgress(hexPath, portPath, fqbn, statusLabel)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("ошибка загрузки: %v", err), w)
				})
			}
		}()
	})

	if err := checkArduinoCLI(); err != nil {
		btnSelectHex.Disable()
		btnSelectPort.Disable()
		btnUpload.Disable()
		statusLabel.SetText("❌ arduino-cli не найден")
	}

	inDialout, _ := checkUserInGroup("dialout")
	inUucp, _ := checkUserInGroup("uucp")
	if !inDialout && !inUucp {
		go func() {
			time.Sleep(500 * time.Millisecond)
			showPermissionWarning(a, w)
		}()
		statusLabel.SetText("⚠️ Нет прав на доступ к портам (нужна группа dialout)")
	}

	content := container.NewVBox(
		header, // Заголовок с иконкой
		widget.NewSeparator(),
		container.NewHBox(btnSelectHex, hexLabel),
		container.NewHBox(btnSelectPort, portLabel),
		widget.NewSeparator(),
		btnUpload,
		statusLabel,
	)

	w.SetContent(content)
	w.ShowAndRun()
}

func selectBoard(board BoardInfo, portPath *string, fqbn *string, portLabel *widget.Label, statusLabel *widget.Label, w fyne.Window) {
	*portPath = board.Port
	portLabel.SetText(board.Port)
	if board.FQBN != "" && strings.Count(board.FQBN, ":") >= 2 {
		*fqbn = board.FQBN
		portLabel.SetText(board.Port + " (" + board.FQBN + ")")
		statusLabel.SetText("Порт выбран: " + board.Port + " (" + board.FQBN + ")")
	} else {
		statusLabel.SetText("Тип платы не определён. Выберите FQBN.")
		showFQBNInputDialog(w, fqbn, statusLabel)
		if *fqbn != "" {
			portLabel.SetText(board.Port + " (" + *fqbn + ")")
			statusLabel.SetText("Порт выбран: " + board.Port + " (" + *fqbn + ")")
		} else {
			portLabel.SetText(board.Port + " (тип не выбран)")
			statusLabel.SetText("FQBN не выбран")
		}
	}
}

func showFQBNInputDialog(w fyne.Window, fqbn *string, statusLabel *widget.Label) {
	commonFQBNs := []string{
		"arduino:avr:uno",
		"arduino:avr:nano",
		"arduino:avr:mega",
		"arduino:avr:leonardo",
		"esp32:esp32:esp32",
		"esp8266:esp8266:generic",
	}

	// Создаём Select
	selectWidget := widget.NewSelect(commonFQBNs, func(s string) {
		*fqbn = s
		statusLabel.SetText("FQBN выбран: " + s)
	})

	// Создаём рамку (контур)
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeWidth = 2
	border.StrokeColor = color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xFF}
	border.CornerRadius = 8

	// Собираем Select с рамкой в стек (БЕЗ отступов)
	selectWithBorder := container.NewStack(
		border,
		selectWidget, // ← Прямо на рамку, без Padded
	)

	// Оборачиваем в контейнер с фиксированной шириной и высотой
	selectWrapper := container.NewHBox(
		selectWithBorder,
	)
	selectWrapper.Resize(fyne.NewSize(600, 80))

	// Создаём Entry для ручного ввода
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Или введите свой FQBN вручную")
	entry.OnSubmitted = func(s string) {
		if s != "" {
			*fqbn = s
			statusLabel.SetText("FQBN введён: " + s)
		}
	}

	var dialogObj *dialog.CustomDialog

	okButton := newButtonWithBorder("✅ OK", func() {
		if *fqbn == "" && entry.Text != "" {
			*fqbn = entry.Text
			statusLabel.SetText("FQBN выбран: " + *fqbn)
		}
		if *fqbn == "" && len(commonFQBNs) > 0 {
			*fqbn = commonFQBNs[0]
			statusLabel.SetText("FQBN выбран: " + *fqbn)
		}
		if dialogObj != nil {
			dialogObj.Hide()
		}
	})

	content := container.NewVBox(
		widget.NewLabel("Выберите или введите FQBN для вашей платы:"),
		selectWrapper,
		widget.NewLabel("или"),
		entry,
		container.NewCenter(okButton),
	)

	dialogObj = dialog.NewCustom("Выбор FQBN", "", content, w)
	dialogObj.Resize(fyne.NewSize(600, 480))
	dialogObj.Show()
}

// getAvailableBoards возвращает список плат через текстовый парсинг
func getAvailableBoards() ([]BoardInfo, error) {
	cmd := exec.Command("arduino-cli", "board", "list")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("ошибка запуска arduino-cli: %v", err)
	}

	lines := strings.Split(out.String(), "\n")
	var boards []BoardInfo
	for _, line := range lines {
		if !strings.Contains(line, "/dev/") && !strings.Contains(line, "COM") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		port := fields[0]
		if !strings.HasPrefix(port, "/dev/") && !strings.HasPrefix(port, "COM") {
			continue
		}
		var fqbn string
		for _, field := range fields {
			if strings.Count(field, ":") >= 2 {
				fqbn = field
				break
			}
		}
		boards = append(boards, BoardInfo{Port: port, FQBN: fqbn})
	}
	return boards, nil
}

func uploadHex(hexPath, portPath, fqbn string) error {
	cmd := exec.Command(
		"arduino-cli",
		"upload",
		"-p", portPath,
		"--fqbn", fqbn,
		"--input-file", hexPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}
	return nil
}