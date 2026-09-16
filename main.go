package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"weather-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/natefinch/lumberjack"
)

// getLogFilePath resolves the absolute path for system logs according to XDG specs.
func getLogFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(homeDir, ".local", "share", "WeatherTUI", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(logDir, "weather_app.log"), nil
}

// initLogger sets up structured JSON logging writing to file with rolling rotation.
func initLogger() *lumberjack.Logger {
	logPath, err := getLogFilePath()
	if err != nil {
		log.Fatalf("Failed to resolve log file path: %v", err)
	}

	logFile := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}

	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(logFile), &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("WeatherTUI service initializing")

	return logFile
}

func main() {
	logFile := initLogger()
	defer logFile.Close()

	p := tea.NewProgram(ui.InitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		slog.Error(fmt.Sprintf("Fatal execution crash: %v", err))
		log.Fatalf("Fatal execution crash: %v", err)
	}
}
