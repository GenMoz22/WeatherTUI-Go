package ui

import (
	"log/slog"
	"strings"

	"weather-tui/internal/config"
	"weather-tui/internal/weather"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ActivePanel int

const (
	SearchPanel ActivePanel = iota
	HistoryPanel
	ForecastPanel
)

type ErrMsg error

type WeatherMsg struct {
	Data  string
	Query string
	W     weather.WeatherResponse
}

type Model struct {
	TextInput          textinput.Model
	Spinner            spinner.Model
	Client             *weather.WeatherClient
	ConfigMgr          *config.ConfigManager
	Weather            weather.WeatherResponse
	CityName           string
	LastQuery          string
	FavoriteCity       string
	RecentLocations    []string
	HistorySelectedRow int
	Err                error
	Loading            bool
	HasData            bool
	Imperial           bool
	ShowHourly         bool
	ActivePanel        ActivePanel
	SelectedRow        int
	TermWidth          int
	TermHeight         int
	ShowHelp           bool
	InitialCity        string
}

// InitialModel constructs the initial TUI state.
func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter city identifier..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 32

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Cyan)

	cfgMgr, err := config.NewConfigManager()
	if err != nil {
		slog.Error("Failed initializing ConfigManager", "error", err)
	}

	imperial := false
	favoriteCity := ""
	recentLocations := make([]string, 0)

	if cfgMgr != nil {
		cfg, err := cfgMgr.Load()
		if err == nil {
			imperial = cfg.Imperial
			favoriteCity = cfg.FavoriteCity
			recentLocations = cfg.RecentLocations
		}
	}

	initialCity := strings.TrimSpace(favoriteCity)
	if initialCity == "" && len(recentLocations) > 0 {
		initialCity = strings.TrimSpace(recentLocations[0])
	}

	return Model{
		TextInput:          ti,
		Spinner:            s,
		Client:             weather.NewWeatherClient(),
		ConfigMgr:          cfgMgr,
		ActivePanel:        SearchPanel,
		Imperial:           imperial,
		ShowHourly:         false,
		FavoriteCity:       favoriteCity,
		RecentLocations:    recentLocations,
		SelectedRow:        0,
		HistorySelectedRow: 0,
		TermWidth:          100,
		TermHeight:         24,
		ShowHelp:           false,
		InitialCity:        initialCity,
	}
}

// Init triggers initial commands on app launch.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink, m.Spinner.Tick}

	if m.InitialCity != "" {
		m.Loading = true
		m.LastQuery = m.InitialCity
		cmds = append(cmds, m.FetchWeatherCmd(m.InitialCity, false))
	}

	return tea.Batch(cmds...)
}

// FetchWeatherCmd executes the async weather query pipeline.
func (m Model) FetchWeatherCmd(city string, forceRefresh bool) tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		func() tea.Msg {
			w, name, err := m.Client.GetWeather(city, forceRefresh)
			if err != nil {
				return ErrMsg(err)
			}
			return WeatherMsg{Data: name, Query: city, W: w}
		},
	)
}

// SaveConfig commits model configuration state to storage.
func (m *Model) SaveConfig() {
	if m.ConfigMgr == nil {
		return
	}
	cfg := config.Config{
		Imperial:        m.Imperial,
		FavoriteCity:    m.FavoriteCity,
		RecentLocations: m.RecentLocations,
	}
	if err := m.ConfigMgr.Save(cfg); err != nil {
		slog.Error("Failed persisting configuration", "error", err)
	}
}

// AddRecentLocation registers a queried location into history.
func (m *Model) AddRecentLocation(location string) {
	m.RecentLocations, m.FavoriteCity = config.UpdateRecentAndFavorite(m.RecentLocations, m.FavoriteCity, location)
	m.SaveConfig()
}

// ToggleFavorite toggles the favorite status for a target location.
func (m *Model) ToggleFavorite(targetCity string) {
	cleanLoc := strings.TrimSpace(targetCity)
	if cleanLoc == "" {
		return
	}

	if strings.EqualFold(m.FavoriteCity, cleanLoc) {
		m.FavoriteCity = ""
	} else {
		m.FavoriteCity = cleanLoc
		filtered := make([]string, 0, len(m.RecentLocations))
		for _, loc := range m.RecentLocations {
			if !strings.EqualFold(loc, cleanLoc) {
				filtered = append(filtered, loc)
			}
		}
		m.RecentLocations = filtered
	}
	m.SaveConfig()
}

// GetHistoryItems aggregates favorite and recent locations for menu rendering.
func (m Model) GetHistoryItems() []string {
	items := make([]string, 0, 4)
	if m.FavoriteCity != "" {
		items = append(items, m.FavoriteCity)
	}
	items = append(items, m.RecentLocations...)
	return items
}
