# LanWatch Go

LanWatch Go is a dependency-free Go network scanner and monitoring tool. It automatically detects devices on your LAN using ping + ARP, tracks state and history in JSON, and provides both a CLI and a modern web dashboard for monitoring and control.

## What It Does

- **Controlled discovery**: Scans configured IPv4 subnets, with optional local-interface detection
- **Device tracking**: Tracks devices by MAC address (preferred) or IP; detects new arrivals, IP changes, and offline status
- **Live monitoring**: Pin critical hosts to watch ping status in real-time (refreshed every 30 seconds)
- **Web dashboard**: Dynamic scan configuration, live search, history, and subnet organization
- **State persistence**: Stores all device data and history in JSON for auditing and replay
- **No root required**: Uses system `ping` and OS ARP tables instead of raw sockets

## How It Discovers Devices

This version uses standard OS tools instead of raw packet libraries:

- Uses the configured IPv4 subnets (by default, only `10.10.65.0/24`)
- Can optionally detect active non-loopback IPv4 interfaces and extract their subnets
- Pings each target IP to trigger ARP/neighbor cache updates
- Reads the OS ARP/neighbor table:
  - macOS/BSD: `arp -an`
  - Linux: `ip neigh show`
- If a MAC address is found → device tracked by MAC
- If only ping response (routed subnet) → device tracked by IP

**Note:** ARP/MAC discovery works best in Layer 2 networks (same VLAN). For routed/remote subnets, ping confirms the IP is alive but the remote MAC may not be visible.

## Build

```bash
cd lanwatchgo
go build -o lanwatchgo .
```

Produces a statically-linked binary with no external dependencies.

## Direct Download

The repository includes a ready-to-run build for macOS on Apple Silicon (arm64):

[Download lanwatchgo for macOS arm64](https://raw.githubusercontent.com/4rji/binarios-go/main/binarios-go/lanwatchgo)

From a terminal:

```bash
curl -L https://raw.githubusercontent.com/4rji/binarios-go/main/binarios-go/lanwatchgo -o lanwatchgo
chmod +x lanwatchgo
```

This binary requires macOS on an Apple Silicon Mac. For other platforms, build it from source.

## Configuration

Copy the example config:

```bash
cp config.example.json config.json
```

Edit `config.json`:

```json
{
  "scan_interval": 60,
  "auto_detect_interfaces": false,
  "interfaces": [],
  "exclude_interfaces": [],
  "extra_subnets": [
    "10.10.65.0/24"
  ],
  "state_path": "lanwatchgo-state.json",
  "ping_timeout_ms": 700,
  "concurrency": 128,
  "max_hosts_per_subnet": 4096
}
```

### Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `scan_interval` | int | Seconds between scans in `watch` mode (default: 60) |
| `auto_detect_interfaces` | bool | Include all active local IPv4 interface subnets (default: false) |
| `interfaces` | array | Specific interfaces to scan, even when auto-detection is disabled |
| `exclude_interfaces` | array | Interfaces to skip (e.g., `["docker0", "veth1234"]`) |
| `extra_subnets` | array | Additional CIDR subnets to scan beyond interface subnets |
| `state_path` | string | Path to JSON state file (default: `lanwatchgo-state.json`) |
| `ping_timeout_ms` | int | Ping timeout in milliseconds (default: 700) |
| `concurrency` | int | Parallel ping workers (default: 128) |
| `max_hosts_per_subnet` | int | Skip subnets larger than this (default: 4096) |

### Notes

- The default configuration scans only `10.10.65.0/24`
- Set `auto_detect_interfaces` to `true` to also scan all active local interfaces
- `exclude_interfaces` filters out virtual interfaces (docker, veth, virbr, tun, tap, vmnet, etc.) automatically—use it only for physical interfaces you want to skip
- `extra_subnets` lists routed or local CIDR networks to scan; you can also change them via the web UI at runtime

## CLI Commands

### Baseline

Record all currently-online devices as "known" so they don't appear as "new" in the dashboard:

```bash
./lanwatchgo baseline
```

Run this once after setting up to establish your baseline. Future scans will only flag truly new devices.

### Scan

Run a single scan and print results to the terminal:

```bash
./lanwatchgo scan
```

Scan only a specific interface:

```bash
./lanwatchgo scan --interface enp0s3
```

Add extra subnets on the fly (without editing config):

```bash
./lanwatchgo scan --subnet 10.10.66.0/24 --subnet 10.10.67.0/24
```

### Watch

Continuous scanning with a fixed interval (from `config.json`):

```bash
./lanwatchgo watch
```

Press Ctrl+C to stop. Logs appear in the terminal.

### List

Show all known devices:

```bash
./lanwatchgo list
```

### History

Show change history for a specific MAC or IP:

```bash
./lanwatchgo history aa:bb:cc:dd:ee:ff
./lanwatchgo history 192.168.1.100
```

History is reverse-chronological (newest first) and limited to recent events.

### Forget

Remove a device from the known device list and clear its history:

```bash
./lanwatchgo forget aa:bb:cc:dd:ee:ff
./lanwatchgo forget 192.168.1.100
```

The device will reappear as "new" if seen again.

### Interfaces

List all detected active physical network interfaces and their subnets:

```bash
./lanwatchgo interfaces
```

Show interfaces for one specific interface:

```bash
./lanwatchgo interfaces --interface enp0s3
```

Virtual interfaces (docker, tun, tap, etc.) are automatically filtered out.

### Serve

Launch the web dashboard:

```bash
./lanwatchgo serve
```

Opens on `http://127.0.0.1:6001` by default.

#### Port Binding Options

Use a different port:

```bash
./lanwatchgo serve 5991
```

Listen on all interfaces:

```bash
./lanwatchgo serve 0.0.0.0 5991
```

Explicit flags:

```bash
./lanwatchgo serve --host 0.0.0.0 --port 5991
```

Logs appear in the terminal where you ran `serve`.

## Web Dashboard

The dashboard provides a live, interactive view of your network and control over scans and monitoring.

### Scan Configuration Panel

Click the **Scan Configuration** dropdown to dynamically change what subnets and interface are being scanned **without restarting**:

- **Interface**: Auto-populated dropdown of detected physical interfaces (ignores docker, virtual, etc.)
- **Subnet 1, 2, 3**: Up to three CIDR subnets to scan (e.g., `192.168.1.0/24`). Leave blank to disable.
- Click **Apply** to activate immediately.

Changes override `config.json` for the lifetime of the server process.

### Monitored Hosts (Ping Status)

Add critical hosts to watch continuously:

1. Type an IP address or MAC in the **+ Monitor** input field at the top
2. Hit Enter or click **+ Monitor**

Each pinned host shows:

- **Status**: Online (🟢) or Offline (🔴)
- **Latency**: Round-trip time in milliseconds (when online)
- **IP & Hostname** (if resolved)
- **Last checked**: ISO 8601 timestamp

The background updates all watched hosts every 30 seconds. Remove a host by clicking the **✕** button.

Limit: up to 20 watched hosts at a time.

### Device Search & Filter

Type in the search box above the device tabs to filter results **live** (no page reload):

- Matches: IP, MAC, hostname, vendor, subnet
- Works across all tabs (Active, History, Subnets, Changed IP, Offline, All)
- Case-insensitive substring match

### Summary Metrics

At the top, quick counts:

- **New 10 min**: Devices first seen in the last 10 minutes
- **Active**: Currently online devices
- **Changed IP**: Devices that moved to a new IP (MAC-tracked devices only)
- **Offline**: Known devices now offline
- **Total**: All devices in the state file

### New Devices Highlight

A dedicated section shows devices discovered in the last 10 minutes:

- Use **Archive current new** to acknowledge them without deleting history
- They stay in the database and history, just no longer flagged as "new"

### Device Tabs

- **Known active**: Online devices with no recent status change
- **History**: Searchable change log; type a MAC or IP to filter
- **Subnets**: Devices grouped by the subnet they were discovered on
- **Changed IP**: Devices whose MAC moved to a different IP (potential device migration or DHCP change)
- **Offline**: Devices last seen online but now unreachable
- **All devices**: Complete device list sorted by last-seen time

### Auto-Scan Controls

Start/stop automatic periodic scans with a configurable interval (in seconds):

- Enter interval, click **Start**
- Click **Stop** to pause
- Scans run in the background; results update the dashboard automatically
- Logs appear in the terminal

### Run Scan Now

Click **Run Scan** to trigger an immediate scan without waiting for auto-scan.

## Data Model

### Device Record

Each device in the state file has:

```json
{
  "key": "aa:bb:cc:dd:ee:ff",
  "key_type": "mac",
  "ip": "192.168.1.50",
  "mac": "aa:bb:cc:dd:ee:ff",
  "vendor": "Apple",
  "hostname": "iphone-user",
  "interface": "en0",
  "subnet": "192.168.1.0/24",
  "first_seen": "2024-01-15T10:30:00Z",
  "last_seen": "2024-01-15T14:22:15Z",
  "last_status": "known"
}
```

- `key`: Primary identifier (MAC for MAC-tracked, IP for IP-only devices)
- `key_type`: "mac" or "ip"
- `last_status`: "new", "known", "changed_ip", "offline"

### History Event

Each scan appends a history entry:

```json
{
  "scanned_at": "2024-01-15T14:22:15Z",
  "key": "aa:bb:cc:dd:ee:ff",
  "key_type": "mac",
  "ip": "192.168.1.50",
  "previous_ip": "192.168.1.49",
  "mac": "aa:bb:cc:dd:ee:ff",
  "vendor": "Apple",
  "hostname": "iphone-user",
  "interface": "en0",
  "subnet": "192.168.1.0/24",
  "status": "changed_ip"
}
```

The `status` field captures what happened at that scan: "new", "known", "changed_ip", or "offline".

## Architecture

### Key Functions

#### `Discover(cfg Config) ([]Observation, []TargetSubnet, error)`

Core discovery function:
1. Builds target subnets from config and, when enabled, auto-detected interfaces
2. Runs parallel ping sweep to find alive IPs
3. Reads OS ARP table
4. Merges ping results + ARP data into Observation objects (IP + MAC + vendor + hostname)
5. Returns all observations and the targets that were scanned

#### `RunScan(cfg Config) (ScanReport, error)`

High-level scan orchestrator:
1. Calls `Discover()`
2. Loads existing state
3. Calls `ApplyScan()` to merge observations into state and classify status changes
4. Saves updated state
5. Returns a `ScanReport` with new/changed/offline/known device lists

#### `ApplyScan(state *State, observations []Observation, targets []TargetSubnet) ScanReport`

Merges observations into the device database:
- New observations: marked "new" if not in state
- MAC + IP change: marked "changed_ip"
- Known devices not seen: marked "offline"
- Everything else: "known"

Appends a `HistoryEvent` to state for each observation.

#### `Serve(cfg Config, host string, port int) error`

Web server:
1. Sets up HTTP handlers for dashboard, API, and control endpoints
2. Starts a background goroutine to ping watched hosts every 30 seconds
3. Serves the HTML dashboard template with live data
4. Supports:
   - `/` – dashboard home (GET)
   - `/scan` – trigger a scan (POST)
   - `/config` – update interface/subnet config (POST)
   - `/watch` – add/remove monitored hosts (POST)
   - `/autoscan` – start/stop auto-scan (POST)
   - `/api/interfaces` – list physical interfaces (GET, JSON)
   - `/archive-new` – mark current new devices as reviewed (POST)

#### `BuildTargets(cfg Config) ([]TargetSubnet, error)`

Assembles the list of subnets to scan:
1. Adds configured interfaces, or auto-detects interfaces when enabled
2. Appends `extra_subnets` from config
3. Validates each subnet size against `max_hosts_per_subnet`
4. Returns deduplicated, sorted list

#### `PingSweep(targets []TargetSubnet, cfg Config) (map[string]bool, error)`

Parallel ping scanner:
1. Enumerates all IPs in target subnets
2. Spawns worker goroutines (concurrency = `cfg.Concurrency`)
3. Each worker pings IPs in parallel
4. Returns map of IPs that responded

#### `ReadARPTable() ([]ARPEntry, error)`

Reads the system ARP table:
- Linux: parses `ip neigh show`
- macOS/BSD: parses `arp -an`

Returns MAC, IP, and interface for each entry.

#### `pingWatchedHostsParallel(mu *sync.Mutex, hosts []*WatchedHost, state *State, timeoutMS int)`

Background function (runs every 30 seconds):
1. Resolves each watched host's IP (if stored by MAC, look up current IP from state)
2. Pings all hosts in parallel
3. Records latency and online/offline status
4. Stores results in the `WatchedHost` objects under mutex protection

#### `listPhysicalInterfaces() ([]InterfaceInfo, error)`

Returns all detected physical (non-virtual) network interfaces with their subnets:
- Filters out: docker*, veth*, virbr*, br-*, tun*, tap*, vmnet*, vboxnet*, awdl*, llw*, utun*, en-bridge, lo
- Returned as JSON for the `/api/interfaces` endpoint

#### `isVirtualInterface(name string) bool`

Helper to identify virtual interfaces by name prefix.

### Data Flow

```
config.json + CLI flags
    ↓
BuildTargets() → []TargetSubnet
    ↓
PingSweep() → map[alive IPs]
    ↓
ReadARPTable() → []ARPEntry
    ↓
BuildObservations() → []Observation (merged IP + MAC + vendor + hostname)
    ↓
ApplyScan() → merge into state, classify changes, append history
    ↓
SaveState() → lanwatchgo-state.json
    ↓
PrintReport() or serve dashboard
```

## Examples

### Scenario: New LAN Setup

1. Build the binary:
   ```bash
   go build -o lanwatchgo .
   ```

2. Create baseline:
   ```bash
   ./lanwatchgo baseline
   ```

3. Start the dashboard:
   ```bash
   ./lanwatchgo serve
   ```

4. Open `http://127.0.0.1:6001` and monitor new devices arriving

### Scenario: Monitor Specific Hosts

1. In the dashboard, add watched hosts by IP or MAC in the **+ Monitor** field
2. Watch the status cards update every 30 seconds
3. Use search to find specific devices in the tables

### Scenario: Scan a Routed Network

1. Edit `config.json`:
   ```json
   "extra_subnets": ["10.0.0.0/24"]
   ```

2. Or add it at runtime in the dashboard **Scan Configuration** panel

3. Scan will ping 10.0.0.0/24 and log results as IP-only entries (no MAC visible for remote networks)

## Notes

- Devices are tracked by MAC when available (preferred—more stable across DHCP changes)
- IP-only devices are tracked by IP address
- Vendor lookup uses a small hardcoded OUI table; real OUI database can be added later
- History is kept indefinitely; state file can grow over time
- No special privileges required (no raw sockets)
- Works on Linux, macOS, and BSD

## Logging

Logs are printed to stderr. Key log entries include:

- `discover:` – network detection steps (interfaces, ping count, ARP entries, observations)
- `scan:` – overall scan progress and summary
- `baseline:` – baseline operation results
- `server:` – dashboard activities (runs, errors)
- `autoscan:` – auto-scan start/stop/tick
- `config:` – runtime config changes
- `archive:` – new device archival

Logs include timing info (e.g., `duration=1.234s`) for performance monitoring.
