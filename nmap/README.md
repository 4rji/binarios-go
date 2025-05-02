# Nmap UI Scanner

This project provides a terminal-based UI for running and visualizing Nmap scans, with HTML report generation.

## Requirements
- Go 1.21 or newer
- Nmap installed on your system
- Run as root (for full Nmap functionality)

## Setup

1. **Clone the repository and enter the directory:**
   ```sh
   git clone <your-repo-url>
   cd <your-repo-directory>
   ```

2. **Install Go dependencies:**
   ```sh
   go mod tidy
   ```

## Usage

### Run the Full UI Scanner
```sh
sudo go run nmap-full.go <CIDR>
```

### Run the Half UI Scanner
```sh
sudo go run nmap-half.go <CIDR>
```

Replace `<CIDR>` with the target network, e.g. `192.168.1.0/24`.

## Output
- The UI will show Nmap processes, scan logs, and a live tail of the port scan results.
- When the scan finishes, an HTML report will be generated in a timestamped directory (e.g. `recon_YYYYMMDD_HHMMSS/report.html` or `test_YYYYMMDD_HHMMSS/report.html`).
- A popup will show the location of the generated report and related log files.

## Notes
- You must run as root for Nmap to perform host discovery and port scanning.
- The UI uses [tview](https://github.com/rivo/tview) and [tcell](https://github.com/gdamore/tcell).
- The HTML report is styled and colorized for easy reading. 