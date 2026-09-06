Terminal User Interface (TUI) weather application written in Go that delivers instant 14-day forecasts and atmospheric telemetry directly to your command line. 

Built on the Bubble Tea framework and styled using Lip Gloss, it orchestrates parallel integrations with Open-Meteo APIs alongside an intelligent, concurrent-safe internal caching engine.

## Core Dependencies
- https://github.com/charmbracelet/bubbletea: The Elm-inspired TUI runtime engine.
- https://github.com/charmbracelet/lipgloss: Layout builder and advanced terminal styling primitives.
- https://github.com/natefinch/lumberjack: Rolling file logger for system tracking.

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
The application integrates with three endpoints from the **Open-Meteo** API ecosystem. Outgoing requests strictly enforce a **10-second timeout boundary**.

1. **Geocoding Engine**
* **Endpoint**: `https://geocoding-api.open-meteo.com/v1/search`
* **Parameters**: `name={city}&count=1&language=it&format=json`
* **Role**: Resolves raw string queries into explicit latitude, longitude, city name, and country metadata.


2. **Meteorological Forecast Engine**
* **Endpoint**: `https://api.open-meteo.com/v1/forecast`
* **Parameters**: `latitude={lat}&longitude={lon}&current=temperature_2m,wind_speed_10m,uv_index,precipitation_probability,relative_humidity_2m&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset&timezone=auto&forecast_days=14`
* **Role**: Retrieves real-time atmospheric metrics and 14-day forecast cycles.


3. **Air Quality Analyzer**
* **Endpoint**: `https://air-quality-api.open-meteo.com/v1/air-quality`
* **Parameters**: `latitude={lat}&longitude={lon}&current=european_aqi`
* **Role**: Queries the European Air Quality Index (AQI) and maps it to UI visual threat indicators (`EXCELLENT` $\le 20$, `POOR` $\le 40$, `CRITICAL` $> 40$).


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

## Project Structure

```text
WeatherTUI-Go/
├── cache.go         # Cache engine (sync.RWMutex, 30m TTL)
├── go.mod           # Go module dependency management
├── go.sum           # Cryptographic checksums for dependencies
├── LICENSE          # Project licensing information
├── main.go          # Bubble Tea model definitions, View rendering, and app entry point
├── README.md        # Project documentation
└── weather.go       # Open-Meteo API client, JSON models, and HTTP request orchestration
```
