**Install Go dependencies:**
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

