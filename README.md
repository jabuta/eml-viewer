# EML Viewer

A fast, cross-platform email viewer for .eml files with full-text search capabilities. Built with Go + HTMX + SQLite.

![Version](https://img.shields.io/badge/version-1.0.0-blue)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

✨ **Simple & Fast**
- Single binary - no dependencies to install
- Auto-opens in your browser
- Indexes 500+ emails in seconds

🔍 **Powerful Search**
- Full-text search with SQLite FTS5
- Fuzzy matching (partial words)
- Search in subject, sender, recipients, and body
- Instant results with highlighted matches

📧 **Complete Email Support**
- HTML and plain text rendering
- Safe HTML display (sandboxed iframes)
- Attachment viewing and downloading
- MIME-encoded header decoding
- Multiple charset support (UTF-8, windows-1252, iso-8859-1)

🖥️ **Cross-Platform**
- Works on Windows, macOS, and Linux
- Responsive web interface
- Works offline - no internet required

## Quick Start

### 1. Download

Download the latest release for your platform:

- **Windows**: `eml-viewer-windows-amd64.exe`
- **macOS (Intel)**: `eml-viewer-macos-intel`
- **macOS (Apple Silicon)**: `eml-viewer-macos-apple`
- **Linux**: `eml-viewer-linux-amd64`

### 2. Place Your .eml Files

Put your .eml files in a folder named `emails` in the same directory as the executable:

```
your-folder/
├── eml-viewer          # or eml-viewer.exe on Windows
└── emails/
    ├── email1.eml
    ├── email2.eml
    └── ...
```

### 3. Run

**Windows**: Double-click `eml-viewer.exe`

**macOS/Linux**: 
```bash
chmod +x eml-viewer
./eml-viewer
```

The application will:
1. Index all .eml files from the `emails` folder
2. Start a local web server on `http://localhost:8080`
3. Automatically open your browser

## Usage

### First Run

On first run, if the `emails` folder doesn't exist, it will be created automatically. Place your .eml files there and restart the application.

### Searching Emails

Use the search bar at the top to search across all your emails:

```
meeting           # Find emails containing "meeting"
invoice john      # Find emails with both "invoice" and "john"
from:sender       # Search by sender (coming soon)
```

Search looks through:
- Email subject
- Sender name and address
- Recipients (To, CC, BCC)
- Email body (text content)

### Viewing Emails

Click on any email in the list to view:
- Full headers (From, To, CC, Date)
- Message body (HTML or plain text)
- Attachments (with download buttons)
- Raw headers (expandable section)

### Re-indexing

If you add new .eml files to the `emails` folder, restart the application to re-index them.

## Building from Source

### Prerequisites

- Go 1.21 or higher
- Git

### Build Steps

```bash
# Clone the repository
git clone https://github.com/yourusername/eml-viewer.git
cd eml-viewer

# Install dependencies
go mod download

# Build
go build -o eml-viewer

# Run
./eml-viewer
```

### Cross-Compilation

Build for different platforms:

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o eml-viewer-windows.exe

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o eml-viewer-macos-intel

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o eml-viewer-macos-apple

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o eml-viewer-linux
```

## Architecture

```
EML Viewer
├── Backend (Go)
│   ├── HTTP Server (chi router)
│   ├── SQLite Database (with FTS5)
│   ├── EML Parser (emersion/go-message)
│   └── File Scanner
│
├── Frontend (HTML + HTMX + TailwindCSS)
│   ├── Email List View
│   ├── Email Detail View
│   └── Search Interface
│
└── Database
    ├── emails table (metadata)
    ├── emails_fts (full-text search)
    └── attachments table (blobs)
```

## Configuration

The application stores its database and configuration in:
- **Windows**: `C:\Users\<username>\.eml-viewer\`
- **macOS/Linux**: `~/.eml-viewer/`

### Default Settings

- **Server Port**: 8080
- **Email Folder**: `./emails`
- **Database**: `~/.eml-viewer/emails.db`

## Troubleshooting

### Browser doesn't open automatically

Manually navigate to: `http://localhost:8080`

### "Port already in use" error

Another application is using port 8080. Either:
1. Stop the other application, or
2. Modify the port in `internal/config/config.go` and rebuild

### Emails not parsing correctly

The application supports most standard .eml files. If you encounter parsing errors:
1. Check the console output for specific errors
2. Verify the .eml files are valid (not corrupted)
3. Check charset support (UTF-8, windows-1252, iso-8859-1 are supported)

### macOS Security Warning

On macOS, you may see "cannot be opened because the developer cannot be verified":
1. Right-click the application
2. Select "Open"
3. Click "Open" in the dialog

Or run from terminal:
```bash
xattr -d com.apple.quarantine eml-viewer-macos-intel
```

### Windows SmartScreen Warning

Click "More info" then "Run anyway"

## Technology Stack

- **Backend**: Go 1.21+
- **Database**: SQLite with FTS5 (modernc.org/sqlite - pure Go)
- **Email Parsing**: emersion/go-message
- **HTTP Server**: go-chi/chi
- **Frontend**: HTMX 1.9 + TailwindCSS 3.x
- **Search**: SQLite FTS5 with fuzzy matching

## Performance

- **Indexing**: ~500 emails/second
- **Search**: <50ms for most queries
- **Memory**: ~50MB typical usage
- **Binary Size**: ~15MB

## Limitations

- Maximum ~100,000 emails recommended
- Attachments stored in database (consider external storage for large collections)
- Single-user application (not designed for concurrent access)

## Roadmap

Future enhancements (post-MVP):

- [ ] Email tagging/labels
- [ ] Advanced filters (date range, sender, has attachments)
- [ ] Export to PDF/CSV
- [ ] Email threading/conversations
- [ ] Multiple folder support
- [ ] Dark mode
- [ ] Keyboard shortcuts

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

MIT License - see LICENSE file for details

## Acknowledgments

- [emersion/go-message](https://github.com/emersion/go-message) - EML parsing
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) - Pure Go SQLite
- [go-chi/chi](https://github.com/go-chi/chi) - HTTP routing
- [HTMX](https://htmx.org) - Dynamic HTML
- [TailwindCSS](https://tailwindcss.com) - Styling

## Support

For issues, questions, or suggestions:
- Open an issue on GitHub
- Email: [your-email@example.com]

---

**Made with ❤️ using Go + HTMX**
