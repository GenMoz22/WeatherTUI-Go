package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/natefinch/lumberjack"
	"log/slog"
)

var (
	cyan      = lipgloss.Color("#00f5d4")
	gray      = lipgloss.Color("#4a5568")
	lightGray = lipgloss.Color("#a0aec0")
	white     = lipgloss.Color("#ffffff")
	red       = lipgloss.Color("#ff0054")
	green     = lipgloss.Color("#7bf1a8")
	yellow    = lipgloss.Color("#ffee32")
	blue      = lipgloss.Color("#00bbf9")
	darkBg    = lipgloss.Color("#1a202c")

	panelTitleStyle = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	labelStyle      = lipgloss.NewStyle().Foreground(gray).Bold(true)
	valueStyle      = lipgloss.NewStyle().Foreground(white)
	highlightStyle  = lipgloss.NewStyle().Foreground(green).Bold(true)

	keyStyle  = lipgloss.NewStyle().Background(lightGray).Foreground(darkBg).Bold(true).Padding(0, 1)
	descStyle = lipgloss.NewStyle().Foreground(lightGray).Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(gray).
			Padding(0, 1)

	activeBoxStyle = boxStyle.Copy().
			BorderForeground(cyan)
)

type errMsg error

type model struct {
	textInput  textinput.Model
	client     *WeatherClient
	weather    WeatherResponse
	cityName   string
	err        error
	loading    bool
	hasData    bool
	termWidth  int
	termHeight int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter city identifier..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 32

	return model{
		textInput:  ti,
		client:     NewWeatherClient(),
		termWidth:  100,
		termHeight: 24,
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

type weatherMsg struct {
	data string
	w    WeatherResponse
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			city := strings.TrimSpace(m.textInput.Value())
			if city == "" {
				return m, nil
			}
			m.loading = true
			m.err = nil
			return m, func() tea.Msg {
				w, name, err := m.client.GetWeather(city)
				if err != nil {
					return errMsg(err)
				}
				return weatherMsg{data: name, w: w}
			}
		}
	case weatherMsg:
		m.loading = false
		m.hasData = true
		m.cityName = msg.data
		m.weather = msg.w
		m.textInput.SetValue("")
		return m, nil
	case errMsg:
		m.loading = false
		m.hasData = false
		m.err = msg
		return m, nil
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	// Calcolo dinamico dello spazio verticale al netto dei margini della UI
	availableHeight := m.termHeight - 4
	if availableHeight < 12 {
		availableHeight = 12
	}

	// Divisione orizzontale fluida (35% sinistra, 65% destra)
	leftWidth := int(float64(m.termWidth) * 0.35)
	if leftWidth < 40 {
		leftWidth = 40 // Mantiene una dimensione minima fissa per evitare troncamenti a sinistra
	}
	rightWidth := m.termWidth - leftWidth - 2

	// Altezze strutturali dei blocchi di sinistra
	const searchBoxHeight = 5
	currentBoxHeight := availableHeight - searchBoxHeight

	// Altezze interne nette rimosse le linee di bordo
	searchInnerHeight := searchBoxHeight - 2
	currentInnerHeight := currentBoxHeight - 2
	forecastInnerHeight := availableHeight - 2

	// ------------------------------------------------------------------------
	// 1. PANNELLO RICERCA
	// ------------------------------------------------------------------------
	searchContent := fmt.Sprintf("%s\n\n%s", panelTitleStyle.Render(" COMPONENT: SEARCH ENGINE "), m.textInput.View())
	searchBox := activeBoxStyle.Width(leftWidth - 2).Height(searchInnerHeight).Render(searchContent)

	// ------------------------------------------------------------------------
	// 2. PANNELLO INFORMAZIONI CORRENTI (Monitor Atmosfera)
	// ------------------------------------------------------------------------
	var currentBox string
	currentHeader := panelTitleStyle.Render(" MONITOR: ATMOSPHERE ")

	if m.hasData {
		aqiVal := m.weather.Current.AirQualityIndex
		var aqiDesc string
		if aqiVal <= 20 {
			aqiDesc = lipgloss.NewStyle().Foreground(green).Render(fmt.Sprintf("%d (EXCELLENT)", aqiVal))
		} else if aqiVal <= 40 {
			aqiDesc = lipgloss.NewStyle().Foreground(yellow).Render(fmt.Sprintf("%d (POOR)", aqiVal))
		} else {
			aqiDesc = lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("%d (CRITICAL)", aqiVal))
		}

		// Stringa compatta dei contenuti analitici
		metricsContent := fmt.Sprintf(
			"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s",
			labelStyle.Render("TARGET:"), "", highlightStyle.Render(m.cityName),
			labelStyle.Render("[TEMP]"), "Temperature:", highlightStyle.Render(fmt.Sprintf("%.1f °C", m.weather.Current.Temperature)),
			labelStyle.Render("[HUMI]"), "Humidity:", valueStyle.Render(fmt.Sprintf("%d%%", m.weather.Current.Humidity)),
			labelStyle.Render("[WIND]"), "Wind Speed:", valueStyle.Render(fmt.Sprintf("%.1f km/h", m.weather.Current.WindSpeed)),
			labelStyle.Render("[SUNR]"), "Sun Rise:", lipgloss.NewStyle().Foreground(blue).Render(m.weather.Current.Sunrise),
			labelStyle.Render("[SUNS]"), "Sun Set:", lipgloss.NewStyle().Foreground(blue).Render(m.weather.Current.Sunset),
			labelStyle.Render("[AQI ]"), "Air Quality:", aqiDesc,
		)

		// Uniamo l'header fisso in alto e lasciamo che Lip Gloss distribuisca verticalmente le righe nello spazio rimanente
		fullCurrentView := currentHeader + "\n\n" + lipgloss.PlaceVertical(currentInnerHeight-3, lipgloss.Top, metricsContent)
		currentBox = boxStyle.Width(leftWidth - 2).Height(currentInnerHeight).Render(fullCurrentView)
	} else {
		placeholderText := "STATUS: SYSTEM IDLE\n\nAwaiting dispatcher query...\nInsert location name above."
		if m.loading {
			placeholderText = "STATUS: PARSING METRICS...\n\nSynchronizing Open-Meteo DB\nStream pipelines active..."
		}
		if m.err != nil {
			placeholderText = lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("[ERR] SYSTEM EXCEPTION:\n%v", m.err))
		}
		currentBox = boxStyle.Width(leftWidth - 2).Height(currentInnerHeight).Render(currentHeader + "\n\n" + placeholderText)
	}

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, searchBox, currentBox)

	// ------------------------------------------------------------------------
	// 3. PANNELLO PREVISIONI 14 GIORNI (Allineamento Rigido Tabelle e Barre)
	// ------------------------------------------------------------------------
	var forecastBox string
	forecastHeader := panelTitleStyle.Render(" METRIC: 14-DAY CORE FORECAST ") + "\n\n"

	// Spazio occupato rigidamente dalle colonne di sinistra del testo (DATE + MAX + MIN + Divisori) = 36 caratteri
	const fixedTableColumnsWidth = 36
	// Sottraiamo la larghezza fissa e i margini per trovare la dimensione esatta della barra
	dynamicBarLength := rightWidth - fixedTableColumnsWidth - 9
	if dynamicBarLength < 10 {
		dynamicBarLength = 10
	}

	// Costruzione dell'Intestazione Tabellare Allineata
	graphHeaderPadding := strings.Repeat(" ", maxInt(0, dynamicBarLength-15))
	tableHeader := lipgloss.NewStyle().Foreground(lightGray).Render(fmt.Sprintf(" DATE       │ MAX TEMP │ MIN TEMP │ PRECIPITATION GRAPH%s", graphHeaderPadding)) + "\n"
	tableDivider := lipgloss.NewStyle().Foreground(gray).Render(strings.Repeat("─", rightWidth-4)) + "\n"

	var tableRows strings.Builder
	if m.hasData {
		for _, day := range m.weather.Daily {
			// Calcolo preciso del riempimento proporzionale al valore percentuale
			filledBlocks := (day.PrecipProbability * dynamicBarLength) / 100

			var barStr strings.Builder
			barStr.WriteString("[")
			for b := 0; b < dynamicBarLength; b++ {
				if b < filledBlocks {
					barStr.WriteString("█")
				} else {
					barStr.WriteString("░")
				}
			}
			barStr.WriteString("]")

			var barStyle lipgloss.Style
			if day.PrecipProbability > 70 {
				barStyle = lipgloss.NewStyle().Foreground(red)
			} else if day.PrecipProbability > 30 {
				barStyle = lipgloss.NewStyle().Foreground(yellow)
			} else {
				barStyle = lipgloss.NewStyle().Foreground(gray)
			}

			renderedBar := barStyle.Render(fmt.Sprintf("%s %3d%%", barStr.String(), day.PrecipProbability))
			// Larghezze fisse imposte tramite formattazione nativa per prevenire slittamenti a destra delle righe
			row := fmt.Sprintf(" %-10s │  %-7.1f │  %-7.1f │ %s", day.Date, day.MaxTemp, day.MinTemp, renderedBar)
			tableRows.WriteString(row + "\n")
		}

		// Tronca le righe in eccesso per evitare lo sfondamento grafico su schermi piccoli
		contentLines := strings.Split(tableRows.String(), "\n")
		maxAllowedRows := forecastInnerHeight - 4
		if len(contentLines) > maxAllowedRows && maxAllowedRows > 0 {
			contentLines = contentLines[:maxAllowedRows]
		}

		forecastBox = boxStyle.Width(rightWidth).Height(forecastInnerHeight).Render(forecastHeader + tableHeader + tableDivider + strings.Join(contentLines, "\n"))
	} else {
		emptyLines := "\n\n\n\n\n\n\n          [WAIT] Pipeline awaiting telemetry input..."
		forecastBox = boxStyle.Width(rightWidth).Height(forecastInnerHeight).Render(forecastHeader + tableHeader + tableDivider + emptyLines)
	}

	mainDashboard := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, forecastBox)

	// ------------------------------------------------------------------------
	// 4. BARRA DELLE SCORCIATOIE INFERIORE
	// ------------------------------------------------------------------------
	statusBar := lipgloss.JoinHorizontal(lipgloss.Left,
		keyStyle.Render("Enter"), descStyle.Render("Search City"),
		keyStyle.Render("Esc"), descStyle.Render("Exit"),
		lipgloss.NewStyle().Foreground(gray).Padding(0, 1).Render("│"),
		lipgloss.NewStyle().Foreground(cyan).Italic(true).Render(fmt.Sprintf("daemon: core-ui v4.6 tmux-ready [res: %dx%d]", m.termWidth, m.termHeight)),
	)

	return "\n" + mainDashboard + "\n\n" + statusBar + "\n"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func initLogger() {
	logFile := &lumberjack.Logger{Filename: "logs/weather_app.log", MaxSize: 10, MaxBackups: 3, MaxAge: 28}
	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(logFile), &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
}

func main() {
	_ = os.Mkdir("logs", os.ModePerm)
	initLogger()
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Fatal execution crash: %v", err)
	}
}