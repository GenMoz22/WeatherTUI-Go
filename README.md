Terminal User Interface (TUI) weather application written in Go that delivers instant 14-day forecasts and atmospheric telemetry directly to your command line. 

Built on the Bubble Tea framework and styled using Lip Gloss, it orchestrates parallel integrations with Open-Meteo APIs alongside an intelligent, concurrent-safe internal caching engine.

## Core Dependencies
- https://github.com/charmbracelet/bubbletea: The Elm-inspired TUI runtime engine.
- https://github.com/charmbracelet/lipgloss: Layout builder and advanced terminal styling primitives.
- https://github.com/natefinch/lumberjack: Rolling file logger for system tracking.

## Installation & Setup
### Prerequisites
Go (version 1.21 or higher)

### Installation Steps
Clone the repository or navigate to your source directory:
```Bash
git@github.com:GenMoz22/WeatherTUI-Go.git
cd WeatherTUI-Go
```

Initialize modules and fetch external libraries:
```Bash
Compile and execute the application:
Bash
go run main.go weather.go cache.go
```

Alternatively, build an optimized standalone binary:
```Bash
go build -o weather-tui
./weather-tui
```
## API Reference
The application interfaces with free-tier endpoints provided by Open-Meteo (no authentication keys required). All outgoing HTTP queries enforce a rigid 10-second request timeout boundary.

1. Geocoding Engine
- Endpoint: https://geocoding-api.open-meteo.com/v1/search
- Parameters: `name={city}&count=1&language=it&format=json`
- Role: Resolves literal query arguments to explicit floating-point latitude, longitude, and territorial origin parameters.

2. Meteorological Forecast Engine
- Endpoint: https://api.open-meteo.com/v1/forecast
- Parameters: `latitude={lat}&longitude={lon}&current=...&daily=...&timezone=auto&forecast_days=14`
- Role: Extracts raw measurements for current atmospheric states alongside 14-day target cycles.

3. Atmospheric Quality Analyzer
- Endpoint: https://air-quality-api.open-meteo.com/v1/air-quality
- Parameters: `latitude={lat}&longitude={lon}&current=european_aqi`
- Role: Evaluates localized air pollution markers mapped directly into customized UI visual threat levels (Excellent, Poor, Critical).

## Error Handling & Resilience
The architecture isolates upstream system exceptions cleanly to prevent application crashes:

- Network Fault Isolation: Gracefully catches failed connection attempts or timed-out API endpoints without interrupting the interactive Bubble Tea event loop.
- Target Missing Intercepts: Detects zero-count coordinate matrices from the Geocoding layer and routes a clear [ERR] SYSTEM EXCEPTION: location target not found alert straight to the UI display panel.
- Graceful API Degradation: Air Quality lookups run independently. If the AQI request fails or drops, the system defaults seamlessly to an implicit 0 value state without breaking the primary weather parser pipeline.

## Logging & Cache
### Concurrent In-Memory Caching
The application implements a custom CacheService backed by a thread-safe read/write mutual exclusion lock (sync.RWMutex) to guarantee stability across multi-threaded asynchronous requests:

- Normalization: Keys are automatically converted to lowercase to match inputs uniformly.
- TTL Window: Hardcoded 30-minute validation window (expirationMinutes: 30).
- Logic Flow: If an entry exists and the timestamp difference remains below the expiration threshold, a cache HIT triggers instantly, eliminating redundant network hops.

### Structured Diagnostic Logging
Logging relies on Go’s standard log/slog structured library tied to a Lumberjack automated file rotator:

- Path: Outputs to logs/weather_app.log.
- Configuration: Maximum filesize is set to 10 MB, maintaining up to 3 older backups over a maximum lifespan of 28 days.
- Telemetry: Logs operational runtime data, tracking pipeline events like cache states (Cache HIT) and network query exceptions.
