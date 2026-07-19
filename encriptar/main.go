package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

const (
	defaultKeyFile = "encryption_key.key"
	magicPrefix    = "GOENC1"
	passwordMode   = 'P'
	keyMode        = 'K'
	saltSize       = 16
	encryptedExt   = ".enc"
	legacyEncExt   = ".encrypted"
)

const (
	colorReset  = "\x1b[0m"
	colorRed    = "\x1b[31m"
	colorYellow = "\x1b[33m"
)

var useColor = term.IsTerminal(int(os.Stderr.Fd()))

func main() {
	log.SetFlags(0)

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		errorf("argument error: %v", err)
		printUsage()
		return
	}

	if opts.showHelp {
		printUsage()
		return
	}

	if !opts.encrypt && !opts.decrypt {
		printUsage()
		return
	}
	if opts.encrypt && opts.decrypt {
		fatalf("choose either -e (encrypt) or -d (decrypt)")
	}
	if opts.target == "" {
		printUsage()
		return
	}

	password := opts.password
	if opts.promptPassword {
		password, err = readPassword("Password: ")
		if err != nil {
			fatalf("could not read password: %v", err)
		}
	}

	absKeyPath, err := filepath.Abs(opts.keyPath)
	if err != nil {
		fatalf("could not resolve key path: %v", err)
	}

	switch {
	case opts.encrypt:
		if password != "" {
			if err := encryptTreeWithPassword(opts.target, password); err != nil {
				fatalf("error during encryption: %v", err)
			}
			return
		}

		key, err := ensureKey(absKeyPath)
		if err != nil {
			fatalf("could not prepare key: %v", err)
		}
		gcm, err := newGCM(key)
		if err != nil {
			fatalf("cipher init failed: %v", err)
		}
		if err := encryptTreeWithKey(opts.target, gcm, absKeyPath); err != nil {
			fatalf("error during encryption: %v", err)
		}
	case opts.decrypt:
		var gcmKey cipher.AEAD
		if opts.keyPathProvided || password == "" {
			if data, err := os.ReadFile(absKeyPath); err == nil {
				if key, err := normalizeKey(data); err == nil {
					if gcmKey, err = newGCM(key); err != nil {
						fatalf("cipher init failed: %v", err)
					}
				} else if password == "" {
					fatalf("key file invalid: %v", err)
				} else {
					warnf("key file invalid, key-based files will fail: %v", err)
				}
			} else if password == "" {
				fatalf("could not read key file and no password provided: %v", err)
			} else if opts.keyPathProvided {
				warnf("key file not readable, key-based files will fail: %v", err)
			}
		}

		if err := decryptTree(opts.target, password, gcmKey); err != nil {
			fatalf("error during decryption: %v", err)
		}
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  encryp -e FileOrDirectory [-p password | -k keyfile]")
	fmt.Println("  encryp -d FileOrDirectory [-p password | -k keyfile]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -e           Encrypt mode (required with -d false)")
	fmt.Println("  -d           Decrypt mode (required with -e false)")
	fmt.Println("  -p [pass]    Password to derive encryption key (prompts if omitted)")
	fmt.Println("  -k <keyfile> Key file path for AES-256 key (default: encryption_key.key)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  encryp -e /tmp/data")
	fmt.Println("  encryp -d /tmp/data -k encryption_key.key")
	fmt.Println("  encryp -e /tmp/data -p")
	fmt.Println("  encryp -e /tmp/data -p mypass")
	fmt.Println("  encryp -d /tmp/data -p")
}

type cliOptions struct {
	encrypt         bool
	decrypt         bool
	promptPassword  bool
	showHelp        bool
	keyPathProvided bool
	password        string
	keyPath         string
	target          string
}

func parseArgs(args []string) (cliOptions, error) {
	opts := cliOptions{keyPath: defaultKeyFile}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-e":
			opts.encrypt = true
		case arg == "-d":
			opts.decrypt = true
		case arg == "-h" || arg == "--help":
			opts.showHelp = true
		case arg == "-p":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				if opts.password != "" || opts.promptPassword {
					return opts, fmt.Errorf("password already provided")
				}
				opts.password = args[i+1]
				i++
			} else {
				if opts.password != "" {
					return opts, fmt.Errorf("password already provided")
				}
				opts.promptPassword = true
			}
		case strings.HasPrefix(arg, "-p="):
			if opts.password != "" || opts.promptPassword {
				return opts, fmt.Errorf("password already provided")
			}
			opts.password = strings.TrimPrefix(arg, "-p=")
			if opts.password == "" {
				opts.promptPassword = true
			}
		case arg == "-k":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for -k")
			}
			opts.keyPathProvided = true
			opts.keyPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown flag %s", arg)
		default:
			if opts.target != "" {
				return opts, fmt.Errorf("unexpected extra argument %q", arg)
			}
			opts.target = arg
		}
	}

	return opts, nil
}

func warnf(format string, args ...any) {
	log.Printf("%s", colorize(fmt.Sprintf(format, args...), colorYellow))
}

func errorf(format string, args ...any) {
	log.Printf("%s", colorize(fmt.Sprintf(format, args...), colorRed))
}

func fatalf(format string, args ...any) {
	log.Fatalf("%s", colorize(fmt.Sprintf(format, args...), colorRed))
}

func colorize(message, color string) string {
	if !useColor {
		return message
	}
	return color + message + colorReset
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		pass, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(pass), nil
	}

	reader := bufio.NewReader(os.Stdin)
	pass, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(pass, "\r\n"), nil
}

func ensureKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return normalizeKey(data)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}

	return key, nil
}

func normalizeKey(key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (got %d)", len(key))
	}
	return key, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func encryptTreeWithKey(root string, gcm cipher.AEAD, skipPath string) error {
	var firstErr error

	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if firstErr == nil {
				firstErr = walkErr
			}
			errorf("cannot access %s: %v", path, walkErr)
			return nil
		}

		if d.IsDir() || isEncryptedName(d.Name()) {
			return nil
		}

		if skipPath != "" {
			absPath, err := filepath.Abs(path)
			if err == nil && absPath == skipPath {
				return nil // Avoid encrypting the key file itself.
			}
		}

		if err := encryptFileWithKey(path, gcm); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			errorf("error encrypting %s: %v", path, err)
		}

		return nil
	}

	_ = filepath.WalkDir(root, walkFn)

	return firstErr
}

func encryptTreeWithPassword(root, password string) error {
	var firstErr error

	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if firstErr == nil {
				firstErr = walkErr
			}
			errorf("cannot access %s: %v", path, walkErr)
			return nil
		}

		if d.IsDir() || isEncryptedName(d.Name()) {
			return nil
		}

		if err := encryptFileWithPassword(path, password); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			errorf("error encrypting %s: %v", path, err)
		}

		return nil
	}

	_ = filepath.WalkDir(root, walkFn)

	return firstErr
}

func decryptTree(root, password string, gcmKey cipher.AEAD) error {
	var firstErr error

	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if firstErr == nil {
				firstErr = walkErr
			}
			errorf("cannot access %s: %v", path, walkErr)
			return nil
		}

		if d.IsDir() || !isEncryptedName(d.Name()) {
			return nil
		}

		if err := decryptFile(path, password, gcmKey); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			errorf("error decrypting %s: %v", path, err)
		}

		return nil
	}

	_ = filepath.WalkDir(root, walkFn)

	return firstErr
}

func encryptFileWithKey(path string, gcm cipher.AEAD) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)

	out := make([]byte, 0, len(magicPrefix)+1+len(nonce)+len(ciphertext))
	out = append(out, []byte(magicPrefix)...)
	out = append(out, keyMode)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	outPath := path + encryptedExt
	if err := os.WriteFile(outPath, out, 0600); err != nil {
		return err
	}

	return nil
}

func encryptFileWithPassword(path, password string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return err
	}

	key := deriveKey(password, salt)
	gcm, err := newGCM(key)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)

	out := make([]byte, 0, len(magicPrefix)+1+saltSize+len(nonce)+len(ciphertext))
	out = append(out, []byte(magicPrefix)...)
	out = append(out, passwordMode)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	outPath := path + encryptedExt
	if err := os.WriteFile(outPath, out, 0600); err != nil {
		return err
	}

	return nil
}

func decryptFile(path, password string, gcmKey cipher.AEAD) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	mode, payload, hasMagic := parseHeader(data)

	switch mode {
	case keyMode:
		if gcmKey == nil {
			return fmt.Errorf("key-based file but no key provided")
		}
		return decryptWithGCM(path, gcmKey, payload)
	case passwordMode:
		if password == "" {
			return fmt.Errorf("password-based file but no password provided")
		}
		if len(payload) < saltSize {
			return fmt.Errorf("ciphertext too short for password mode: %s", path)
		}
		salt := payload[:saltSize]
		body := payload[saltSize:]
		key := deriveKey(password, salt)
		gcm, err := newGCM(key)
		if err != nil {
			return err
		}
		return decryptWithGCM(path, gcm, body)
	default:
		// Fallback for old format without header: expects key-based nonce+ciphertext.
		if !hasMagic && gcmKey != nil {
			return decryptWithGCM(path, gcmKey, data)
		}
		if !hasMagic {
			return fmt.Errorf("unknown file format and no key available: %s", path)
		}
		return fmt.Errorf("unsupported mode byte %q in %s", mode, path)
	}
}

func parseHeader(data []byte) (byte, []byte, bool) {
	prefix := []byte(magicPrefix)
	if len(data) < len(prefix)+1 {
		return 0, data, false
	}
	if !bytes.HasPrefix(data, prefix) {
		return 0, data, false
	}
	return data[len(prefix)], data[len(prefix)+1:], true
}

func decryptWithGCM(path string, gcm cipher.AEAD, payload []byte) error {
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return fmt.Errorf("ciphertext too short: %s", path)
	}

	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	outPath, err := decryptOutputPath(path)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, plain, 0600); err != nil {
		return err
	}

	return nil
}

func deriveKey(password string, salt []byte) []byte {
	hasher := sha256.New()
	_, _ = hasher.Write(salt)
	_, _ = hasher.Write([]byte(password))
	return hasher.Sum(nil)
}

func isEncryptedName(name string) bool {
	return strings.HasSuffix(name, encryptedExt) || strings.HasSuffix(name, legacyEncExt)
}

func decryptOutputPath(path string) (string, error) {
	if strings.HasSuffix(path, encryptedExt) {
		return strings.TrimSuffix(path, encryptedExt), nil
	}
	if strings.HasSuffix(path, legacyEncExt) {
		return strings.TrimSuffix(path, legacyEncExt), nil
	}
	return "", fmt.Errorf("file does not have %s or %s suffix: %s", encryptedExt, legacyEncExt, path)
}
