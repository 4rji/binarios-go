Instalacion desde GitHub:

```bash
go install github.com/4rji/binarios-go/sitemirror@latest
```

Para levantar un servidor ejecuta php -S 0.0.0.0:8000

# Go Website Mirroring Tool

## Description
Simple Go crawler that mirrors a local web server. It downloads pages and assets (HTML, CSS, JS, images) only from the specified host and saves all visited URLs.

## Installation
```bash
go mod init mirror
go get github.com/gocolly/colly/v2
```

## Usage
Run the crawler by specifying an IP, host, or full URL:

```bash
go run main.go 10.129.95.241
# or
go run main.go http://10.129.95.241
```

Output will be created in:

```
mirror_<host>/
├── index.html
├── settings.php
├── assets/...
└── urls.txt
```

`urls.txt` contains all discovered links from the server.

## Serving the mirrored site (IMPORTANT)
If the site contains `.php` files, do NOT use `python3 -m http.server`. Python only serves files and does not execute PHP, so browsers may download `.php` instead of rendering them.

Use PHP’s built-in server instead:

```bash
cd mirror_<host>
php -S 0.0.0.0:8000
```

Then open:

```
http://<your-ip>:8000/
```

## Notes
- Only follows links from the same host/IP.
- Designed for local servers / labs / internal environments.
- Dynamic PHP logic requires PHP to be available locally.
