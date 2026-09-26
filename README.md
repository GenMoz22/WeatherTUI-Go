Terminal User Interface (TUI) weather application written in Go that delivers instant 14-day forecasts and atmospheric telemetry directly to your command line. 

Built on the Bubble Tea framework and styled using Lip Gloss, it orchestrates parallel integrations with Open-Meteo APIs alongside an concurrent-safe internal caching engine.

## Key Features
- **Automatic Startup Target**: Automatically resolves and loads weather metrics for your favorite city upon launch, gracefully falling back to your most recent search.
- **Persistent Configuration**: Persists settings such as preferred unit system (°C/°F, km/h/mph), favorite city, and search history atomically in `~/.local/share/WeatherTUI/config.json`.
- **Resilient Fallbacks**: Non-blocking European Air Quality Index (AQI) fetch with standard fallback and error boundary handling.
- **14-Day Visual Forecasts**: Relative precipitation probability density bars with dynamic color thresholds.
- **24-Hour Thermal Trend**: Toggleable hourly temperature trend curve rendered via ASCII sparklines.
- **Coordinates & Reverse Geocoding**: Direct coordinate input parsing (e.g. `41.9028, 12.4964`) with automated reverse geocoding lookup.
- **Apparent Temperature Calculation**: Calculates real-time perceived temperature ("feels like") based on the Steadman / Australian Apparent Temperature model when atmospheric variance is significant.
- **Lunar Telemetry**: Real-time lunar phase tracking with visual phase representation and illumination percentage.
- **In-Memory Cache**: Thread-safe TTL cache layer (`sync.RWMutex`, 30 min expiration) avoiding redundant API requests.
- **Recent Searches History**: Dedicated panel tracking recent lookups with quick reload capability (`Enter`).
- **Dynamic System Locale Detection**: Automatically detects user's system language (`LANG`, `LC_ALL`, `LC_MESSAGES`) to query localized location names via geocoding with English fallback.
- **Async Visual Feedback**: Animated Lip Gloss-styled spinners integrated with Bubble Tea event loops during network dispatches.
- **Contextual Help Overlay**: Modal view accessible via `?` displaying keybindings and navigation controls.

## Core Dependencies
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea): Elm-inspired TUI runtime engine.
- [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss): Layout builder and advanced terminal styling primitives.
- [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles): Terminal UI components (Input fields, Spinners).
- [natefinch/lumberjack](https://github.com/natefinch/lumberjack): Rolling file logger for system tracking.

## Keybindings & Navigation

| Key | Context | Action |
| :--- | :--- | :--- |
| `Tab` | Global | Cycle focus across panels (Search → Recent → Forecast) |
| `1` / `2` / `3` | Global | Jump directly to Search, Recent & Favorites, or Forecast panel |
| `?` | Global | Toggle keybindings help overlay modal |
| `Esc` | Global | Unfocus search panel or return to dashboard |
| `Ctrl+C` | Global | Force terminate application |
| `Enter` | Search | Execute location query |
| `j` / `k` or `Down` / `Up` | Recent / Forecast | Navigate rows in active panel |
| `Enter` | Recent | Load selected historical location |
| `p` | Recent / Forecast | Toggle favorite status for selected/active location |
| `v` | Forecast | Toggle 24-hour hourly temperature trend sparkline |
| `u` | Forecast | Toggle unit measurement system (°C / °F, km/h / mph) |
| `r` | Forecast | Force refresh weather telemetry (bypass cache) |

---

## Installation & Setup

### Prerequisites
- **Go**: Version `1.21` or higher.

### Quick Start

1. **Clone the repository**:

```bash
git clone git@github.com:GenMoz22/WeatherTUI-Go.git
cd WeatherTUI-Go
```

2. **Build and Run**:
You can run the project directly:

```bash
go run .
```

Or build a standalone binary:

```bash
go build -o weather-tui
./weather-tui
```
---

## API Reference & Data Pipeline
The application integrates with three endpoints from the **Open-Meteo** API ecosystem. All outgoing network requests enforce a strict **10-second timeout boundary**.

1. **Geocoding Engine**
* **Endpoint**: `https://geocoding-api.open-meteo.com/v1/search` and `https://geocoding-api.open-meteo.com/v1/get`
* **Parameters**: `name={city}&count=1&language={sys_lang}&format=json` / `latitude={lat}&longitude={lon}`
* **Role**: Resolves search queries into coordinates and localized location metadata. Supports direct coordinate queries with reverse geocoding fallback.


2. **Meteorological Forecast Engine**
* **Endpoint**: `https://api.open-meteo.com/v1/forecast`
* **Parameters**: `latitude={lat}&longitude={lon}&current=temperature_2m,wind_speed_10m,wind_direction_10m,uv_index,precipitation_probability,relative_humidity_2m&hourly=temperature_2m&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset&timezone=auto&forecast_days=14`
* **Role**: Retrieves real-time atmospheric telemetry, 24-hour hourly temperatures, and 14-day daily forecast data.


3. **Air Quality Analyzer**
* **Endpoint**: `https://air-quality-api.open-meteo.com/v1/air-quality`
* **Parameters**: `latitude={lat}&longitude={lon}&current=european_aqi`
* **Role**: Queries the European Air Quality Index (AQI) mapped to visual threat indicators (`EXCELLENT` $\le 20$, `POOR` $\le 40$, `CRITICAL` $> 40$).

## Architectural Systems

### Concurrent In-Memory Cache (`CacheService`)
To minimize network latency and respect public rate limits, weather data is cached internally:

* **Concurrency Control**: Read/Write mutual exclusion (`sync.RWMutex`) guarantees memory safety across asynchronous Bubble Tea commands.
* **Normalization**: Query strings are automatically sanitized to lowercase.
* **TTL Validation**: Entries older than 30 minutes trigger a cache `MISS` and execute a fresh network call; otherwise, a cache `HIT` returns data instantly.

### Structured Diagnostic Logging (`slog` + `lumberjack`)
Logs are formatted in **JSON** via Go's standard `log/slog` and written concurrently to file and console outputs using `io.MultiWriter`.

* **Log Path (XDG Spec)**: `~/.local/share/WeatherTUI/logs/weather_app.log`
* **Rotation Rules**:
* **Max File Size**: 10 MB per file.
* **Backups**: Retains up to 3 old log files.
* **Lifespan**: Rotates out files older than 30 days.
* **Compression**: Rotated backups are compressed (`.gz`).


## Error Handling & Resilience
* **Network Fault Isolation**: Catches connectivity errors or API timeouts gracefully, presenting an actionable `[ERR] SYSTEM EXCEPTION` message in the status viewport without crashing the TUI event loop.
* **Unresolved Location Target**: Handles zero-result queries from the geocoding service and notifies the user via status banners.
* **Non-blocking AQI Failure**: If the European AQI lookup fails or returns bad data, the core weather parser degrades gracefully, defaulting the AQI metric to `0` without dropping temperature or forecast payloads.
* **Terminal Dimension Guard**: Displays a clean warning overlay when terminal dimensions drop below 30x10 characters to prevent layout corruption, alongside a dynamic single-column layout for small viewports.

## Project Structure

```text
weather-tui/
├── main.go               # Entry point: flag parsing, logger setup, and Bubbletea initialization
├── go.mod
├── go.sum
├── internal/
│   ├── config/           # User configuration management
│   │   └── config.go
│   ├── ui/               # Bubbletea user interface components and logic
│   │   ├── helpers.go    # Conversion and formatting utilities (temperatures, UV, etc.)
│   │   ├── model.go      # Bubbletea model structure and constants
│   │   ├── styles.go     # Lipgloss styles and color palette definitions
│   │   └── update.go     # Event and state management (Update)
│   │   └── view.go       # Graphical rendering logic and help overlay (View)
│   └── weather/          # HTTP API client and weather data models
│       └── client.go
```
