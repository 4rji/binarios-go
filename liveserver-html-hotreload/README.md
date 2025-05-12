# GoLiveReload

Lightweight live-reload tool written in Go. It opens a local HTML file in Chromium/Chrome and automatically reloads it when the file is modified.

---

## Features

* Opens an HTML file using embedded Chromium (via Rod)
* Watches for file changes using fsnotify
* Auto-reloads the browser view on save
* Minimal dependencies, cross-platform support (macOS, Linux, Windows)

---

## Installation

1. **Clone this repository:**

```bash
git clone https://github.com/4rji/HTMLHotReload-go.git
cd HTMLHotReload-go
```

2. **Initialize Go module and install dependencies:**

```bash
go mod tidy
```

3. **Build the binary:**

```bash
go build -o liveserver main.go
```

---

## Usage

```bash
./livego path/to/your/file.html
```

Example:

```bash
./livego ruta/pagina.html
```

This will open the file in a visible Chromium window and reload automatically when the file is changed.

---

## File Structure

```
golivereload/
├── main.go             # Go source code
├── go.mod              # Module definition
├── go.sum              # Dependency hashes
└── ruta/
    └── pagina.html     # Example HTML file
```

---

## Requirements

* Go 1.18 or later
* Chromium or Google Chrome installed
* Internet connection on first `go mod tidy` to fetch dependencies

---

## Notes

* The tool launches a visible Chromium instance using Rod.
* If Chromium is not found, it will try to download a portable version.
* Tested on macOS, Linux, and Windows.

---

## License

MIT

