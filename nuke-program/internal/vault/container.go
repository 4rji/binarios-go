package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	fileMagic         = "NUKEVLT1"
	fileVersion       = 1
	maxHeaderBytes    = 64 << 10
	MaxPlaintextBytes = 256 << 20
	maxContainerBytes = MaxPlaintextBytes + maxHeaderBytes + 1<<20
)

var (
	ErrWrongPassword = errors.New("wrong password")
	ErrNukePassword  = errors.New("nuke password supplied")
	ErrInvalidVault  = errors.New("invalid NukeVault container")
)

type KDFParams struct {
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memoryKiB"`
	Threads   uint8  `json:"threads"`
}

var DefaultKDFParams = KDFParams{Time: 3, MemoryKiB: 64 * 1024, Threads: 1}

type Header struct {
	Version          int       `json:"version"`
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"createdAt"`
	KDF              KDFParams `json:"kdf"`
	NormalSalt       string    `json:"normalSalt"`
	NormalNonce      string    `json:"normalNonce"`
	WrappedMasterKey string    `json:"wrappedMasterKey"`
	NukeSalt         string    `json:"nukeSalt"`
	NukeNonce        string    `json:"nukeNonce"`
	WrappedNukeToken string    `json:"wrappedNukeToken"`
	PayloadNonce     string    `json:"payloadNonce"`
}

type VaultInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
}

type Unlocked struct {
	Header    Header
	MasterKey []byte
	Snapshot  Snapshot
}

func Create(root, name, password, nukePassword string, params KDFParams) (VaultInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 || strings.ContainsAny(name, "\x00\r\n") {
		return VaultInfo{}, errors.New("vault name must contain 1 to 64 printable characters")
	}
	if len(password) < 8 || len(nukePassword) < 8 {
		return VaultInfo{}, errors.New("both passwords must contain at least 8 characters")
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(nukePassword)) == 1 {
		return VaultInfo{}, errors.New("unlock and nuke passwords must be different")
	}
	if err := validateKDF(params); err != nil {
		return VaultInfo{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return VaultInfo{}, fmt.Errorf("create vault directory: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return VaultInfo{}, fmt.Errorf("protect vault directory: %w", err)
	}

	idBytes, err := randomBytes(16)
	if err != nil {
		return VaultInfo{}, err
	}
	id := hex.EncodeToString(idBytes)
	header := Header{
		Version:   fileVersion,
		ID:        id,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		KDF:       params,
	}

	masterKey, err := randomBytes(32)
	if err != nil {
		return VaultInfo{}, err
	}
	defer zero(masterKey)

	normalSalt, err := randomBytes(16)
	if err != nil {
		return VaultInfo{}, err
	}
	normalKey := deriveKey(password, normalSalt, params)
	normalNonce, wrappedMasterKey, err := seal(normalKey, masterKey, aad(id, "unlock"))
	zero(normalKey)
	if err != nil {
		return VaultInfo{}, err
	}

	nukeSalt, err := randomBytes(16)
	if err != nil {
		return VaultInfo{}, err
	}
	nukeKey := deriveKey(nukePassword, nukeSalt, params)
	nukeNonce, wrappedNukeToken, err := seal(nukeKey, nukeMarker(id), aad(id, "nuke"))
	zero(nukeKey)
	if err != nil {
		return VaultInfo{}, err
	}

	header.NormalSalt = encode(normalSalt)
	header.NormalNonce = encode(normalNonce)
	header.WrappedMasterKey = encode(wrappedMasterKey)
	header.NukeSalt = encode(nukeSalt)
	header.NukeNonce = encode(nukeNonce)
	header.WrappedNukeToken = encode(wrappedNukeToken)

	snapshot := InitialSnapshot(name)
	plain, err := encodeSnapshot(snapshot)
	if err != nil {
		return VaultInfo{}, err
	}
	payloadNonce, ciphertext, err := seal(masterKey, plain, aad(id, "payload"))
	zero(plain)
	if err != nil {
		return VaultInfo{}, err
	}
	header.PayloadNonce = encode(payloadNonce)

	containerPath := filepath.Join(root, id+".nukevol")
	if err := writeContainerExclusive(containerPath, header, ciphertext); err != nil {
		return VaultInfo{}, err
	}
	info, err := os.Stat(containerPath)
	if err != nil {
		return VaultInfo{}, err
	}
	return VaultInfo{ID: id, Name: name, CreatedAt: header.CreatedAt, Size: info.Size()}, nil
}

func Authenticate(containerPath, password string) (*Unlocked, error) {
	header, ciphertext, err := readContainer(containerPath, true)
	if err != nil {
		return nil, err
	}

	normalSalt, err := decode(header.NormalSalt)
	if err != nil {
		return nil, ErrInvalidVault
	}
	normalNonce, err := decode(header.NormalNonce)
	if err != nil {
		return nil, ErrInvalidVault
	}
	wrappedMasterKey, err := decode(header.WrappedMasterKey)
	if err != nil {
		return nil, ErrInvalidVault
	}
	normalKey := deriveKey(password, normalSalt, header.KDF)
	masterKey, normalErr := open(normalKey, normalNonce, wrappedMasterKey, aad(header.ID, "unlock"))
	zero(normalKey)

	nukeSalt, err := decode(header.NukeSalt)
	if err != nil {
		zero(masterKey)
		return nil, ErrInvalidVault
	}
	nukeNonce, err := decode(header.NukeNonce)
	if err != nil {
		zero(masterKey)
		return nil, ErrInvalidVault
	}
	wrappedNukeToken, err := decode(header.WrappedNukeToken)
	if err != nil {
		zero(masterKey)
		return nil, ErrInvalidVault
	}
	nukeKey := deriveKey(password, nukeSalt, header.KDF)
	nukeToken, nukeErr := open(nukeKey, nukeNonce, wrappedNukeToken, aad(header.ID, "nuke"))
	zero(nukeKey)

	if normalErr != nil {
		zero(masterKey)
		if nukeErr == nil && subtle.ConstantTimeCompare(nukeToken, nukeMarker(header.ID)) == 1 {
			zero(nukeToken)
			return nil, ErrNukePassword
		}
		zero(nukeToken)
		return nil, ErrWrongPassword
	}
	zero(nukeToken)
	if len(masterKey) != 32 {
		zero(masterKey)
		return nil, ErrInvalidVault
	}

	payloadNonce, err := decode(header.PayloadNonce)
	if err != nil {
		zero(masterKey)
		return nil, ErrInvalidVault
	}
	plain, err := open(masterKey, payloadNonce, ciphertext, aad(header.ID, "payload"))
	if err != nil {
		zero(masterKey)
		return nil, fmt.Errorf("decrypt vault payload: %w", ErrInvalidVault)
	}
	snapshot, err := decodeSnapshot(plain)
	zero(plain)
	if err != nil {
		zero(masterKey)
		return nil, err
	}
	return &Unlocked{Header: header, MasterKey: masterKey, Snapshot: snapshot}, nil
}

func SaveSnapshot(containerPath string, header Header, masterKey []byte, snapshot Snapshot) error {
	if len(masterKey) != 32 {
		return errors.New("invalid master key")
	}
	plain, err := encodeSnapshot(snapshot)
	if err != nil {
		return err
	}
	nonce, ciphertext, err := seal(masterKey, plain, aad(header.ID, "payload"))
	zero(plain)
	if err != nil {
		return err
	}
	header.PayloadNonce = encode(nonce)
	return replaceContainer(containerPath, header, ciphertext)
}

func List(root string) ([]VaultInfo, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []VaultInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	result := make([]VaultInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || filepath.Ext(entry.Name()) != ".nukevol" {
			continue
		}
		containerPath := filepath.Join(root, entry.Name())
		header, _, err := readContainer(containerPath, false)
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, VaultInfo{ID: header.ID, Name: header.Name, CreatedAt: header.CreatedAt, Size: info.Size()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func FindPath(root, id string) (string, error) {
	if len(id) != 32 {
		return "", ErrInvalidVault
	}
	if _, err := hex.DecodeString(id); err != nil {
		return "", ErrInvalidVault
	}
	candidate := filepath.Join(root, id+".nukevol")
	header, _, err := readContainer(candidate, false)
	if err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare([]byte(header.ID), []byte(id)) != 1 {
		return "", ErrInvalidVault
	}
	return candidate, nil
}

func ReadInfo(containerPath string) (VaultInfo, error) {
	header, _, err := readContainer(containerPath, false)
	if err != nil {
		return VaultInfo{}, err
	}
	info, err := os.Stat(containerPath)
	if err != nil {
		return VaultInfo{}, err
	}
	return VaultInfo{ID: header.ID, Name: header.Name, CreatedAt: header.CreatedAt, Size: info.Size()}, nil
}

func readContainer(containerPath string, includePayload bool) (Header, []byte, error) {
	f, info, err := openRegular(containerPath)
	if err != nil {
		return Header{}, nil, err
	}
	defer f.Close()
	if info.Size() < int64(len(fileMagic)+4) || info.Size() > maxContainerBytes {
		return Header{}, nil, ErrInvalidVault
	}
	magic := make([]byte, len(fileMagic))
	if _, err := io.ReadFull(f, magic); err != nil || string(magic) != fileMagic {
		return Header{}, nil, ErrInvalidVault
	}
	var headerLen uint32
	if err := binary.Read(f, binary.BigEndian, &headerLen); err != nil || headerLen == 0 || headerLen > maxHeaderBytes {
		return Header{}, nil, ErrInvalidVault
	}
	headerJSON := make([]byte, headerLen)
	if _, err := io.ReadFull(f, headerJSON); err != nil {
		return Header{}, nil, ErrInvalidVault
	}
	var header Header
	decoder := json.NewDecoder(bytes.NewReader(headerJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&header); err != nil {
		return Header{}, nil, ErrInvalidVault
	}
	if err := validateHeader(header); err != nil {
		return Header{}, nil, err
	}
	if !includePayload {
		return header, nil, nil
	}
	ciphertext, err := io.ReadAll(io.LimitReader(f, maxContainerBytes+1))
	if err != nil || len(ciphertext) == 0 || len(ciphertext) > maxContainerBytes {
		return Header{}, nil, ErrInvalidVault
	}
	return header, ciphertext, nil
}

func validateHeader(header Header) error {
	if header.Version != fileVersion || len(header.ID) != 32 || header.Name == "" || len(header.Name) > 64 {
		return ErrInvalidVault
	}
	if _, err := hex.DecodeString(header.ID); err != nil {
		return ErrInvalidVault
	}
	if err := validateKDF(header.KDF); err != nil {
		return ErrInvalidVault
	}
	for _, value := range []string{
		header.NormalSalt, header.NormalNonce, header.WrappedMasterKey,
		header.NukeSalt, header.NukeNonce, header.WrappedNukeToken, header.PayloadNonce,
	} {
		if value == "" {
			return ErrInvalidVault
		}
	}
	return nil
}

func validateKDF(params KDFParams) error {
	if params.Time < 1 || params.Time > 10 || params.MemoryKiB < 8*1024 || params.MemoryKiB > 256*1024 || params.Threads < 1 || params.Threads > 16 {
		return errors.New("unsupported Argon2id parameters")
	}
	return nil
}

func writeContainerExclusive(containerPath string, header Header, ciphertext []byte) error {
	data, err := marshalContainer(header, ciphertext)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(containerPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	removeOnFailure := true
	defer func() {
		_ = f.Close()
		if removeOnFailure {
			_ = os.Remove(containerPath)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write container: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync container: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close container: %w", err)
	}
	removeOnFailure = false
	return syncDir(filepath.Dir(containerPath))
}

func replaceContainer(containerPath string, header Header, ciphertext []byte) error {
	if filepath.Ext(containerPath) != ".nukevol" {
		return ErrInvalidVault
	}
	data, err := marshalContainer(header, ciphertext)
	if err != nil {
		return err
	}
	parent := filepath.Dir(containerPath)
	tmp, err := os.CreateTemp(parent, ".nukevol-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, containerPath); err != nil {
		return err
	}
	cleanup = false
	return syncDir(parent)
}

func marshalContainer(header Header, ciphertext []byte) ([]byte, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	if len(headerJSON) > maxHeaderBytes || len(ciphertext) == 0 || len(ciphertext) > maxContainerBytes {
		return nil, ErrInvalidVault
	}
	var result bytes.Buffer
	result.Grow(len(fileMagic) + 4 + len(headerJSON) + len(ciphertext))
	result.WriteString(fileMagic)
	if err := binary.Write(&result, binary.BigEndian, uint32(len(headerJSON))); err != nil {
		return nil, err
	}
	result.Write(headerJSON)
	result.Write(ciphertext)
	return result.Bytes(), nil
}

func openRegular(target string) (*os.File, os.FileInfo, error) {
	before, err := os.Lstat(target)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, ErrInvalidVault
	}
	f, err := os.Open(target)
	if err != nil {
		return nil, nil, err
	}
	after, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		f.Close()
		return nil, nil, ErrInvalidVault
	}
	return f, after, nil
}

func deriveKey(password string, salt []byte, params KDFParams) []byte {
	return argon2.IDKey([]byte(password), salt, params.Time, params.MemoryKiB, params.Threads, 32)
}

func seal(key, plaintext, associatedData []byte) ([]byte, []byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, nil, err
	}
	nonce, err := randomBytes(aead.NonceSize())
	if err != nil {
		return nil, nil, err
	}
	return nonce, aead.Seal(nil, nonce, plaintext, associatedData), nil
}

func open(key, nonce, ciphertext, associatedData []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, ErrInvalidVault
	}
	return aead.Open(nil, nonce, ciphertext, associatedData)
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("AES-256 requires a 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func nukeMarker(id string) []byte {
	sum := sha256.Sum256([]byte("NukeVault destructive credential:" + id))
	return sum[:]
}

func aad(id, purpose string) []byte {
	return []byte(fileMagic + ":" + id + ":" + purpose)
}

func randomBytes(size int) ([]byte, error) {
	result := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, result); err != nil {
		return nil, fmt.Errorf("secure random: %w", err)
	}
	return result, nil
}

func encode(value []byte) string {
	return base64.RawStdEncoding.EncodeToString(value)
}

func decode(value string) ([]byte, error) {
	return base64.RawStdEncoding.DecodeString(value)
}

func zero(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
