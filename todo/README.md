# Todo - Script Selector

A Go-based script selector tool that helps you find and execute scripts from your bin directory with intelligent search capabilities.

## Features

- **Intelligent Search**: Search scripts by name or description with word boundary matching
- **Multiple Sources**: Searches in script files, README.md, and descriptions.json
- **Fuzzy Finder**: Uses fzf for interactive script selection
- **Detailed Descriptions**: Shows detailed information about each script
- **Script Execution**: Execute scripts directly or copy commands to clipboard

## Installation

### Using go install (Recommended)

```bash
go install github.com/4rji/binarios-go/todo@latest
```

### From Source

```bash
git clone https://github.com/4rji/binarios-go.git
cd binarios-go/todo
go build -o todo
```

## Usage

### Basic Usage

```bash
# List all available scripts
todo

# Search for scripts containing "tunnel"
todo -s tunnel

# Search with long flag
todo --search tunnel
```

### Interactive Mode

When you run `todo` without arguments, you'll see an interactive interface where you can:

- Type to search for scripts
- Use F5 to switch to content search mode
- Press Enter to select a script
- View detailed descriptions and execute scripts

## Search Features

The tool searches in multiple sources:

1. **Script Files**: Searches the content of script files in `/opt/4rji/bin/`
2. **README.md**: Searches script names and descriptions from the README file
3. **Word Boundaries**: Uses precise word boundary matching to avoid false positives

## Requirements

- Go 1.24.1 or later
- fzf (for interactive selection)
- Scripts located in `/opt/4rji/bin/`
- README.md in `/opt/4rji/bin/README.md`
- Optional: descriptions.json in `/opt/4rji/bin/descriptions.json`

## Configuration

The tool expects the following directory structure:

```
/opt/4rji/bin/
├── README.md
├── .todoignore (optional)
├── descriptions.json (optional)
└── [your scripts]
```

### Excluding scripts

To hide binaries from the program, create `/opt/4rji/bin/.todoignore` with one binary name per line:

```text
nmapX
pingm
winPEASx64.exe
```

Blank lines and lines starting with `#` are ignored.

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is part of the 4rji/binarios-go collection.
