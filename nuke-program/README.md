# NukeVault

NukeVault is a deliberately destructive encrypted vault for controlled digital-forensics labs. It is a Go and Wails 3 desktop application for macOS and Linux. It does not require FUSE: an unlocked vault is exposed through a loopback-only WebDAV server and mounted with the operating system's user-space WebDAV integration when available.

> **Warning:** NukeVault is intentionally capable of irreversible data loss. Use only disposable classroom evidence. Never store the only copy of real data in a `.nukevol` container.

## Destructive challenge

The unlock screen is enforced by the Go backend, not only by JavaScript:

- the normal password unlocks the vault;
- the dedicated Nuke password immediately starts destruction;
- three incorrect passwords start destruction; and
- entering nothing for 10 seconds starts destruction.

After an incorrect attempt, a new 10-second window begins. The timer starts only after the UI has loaded and explicitly arms the selected vault.

The Nuke sequence:

1. Stops the challenge and unmounts an active WebDAV session if necessary.
2. Overwrites the first MiB containing the encrypted key slots and header with cryptographically random data.
3. Overwrites the complete container with random data.
4. Overwrites the complete container with zeros.
5. Flushes writes and deletes the container.

After a successful unlock, the UI also offers **Destroy Vault**. It requires both a confirmation dialog and the exact phrase `DELETE <vault name>` before starting the same Nuke sequence. The backend refuses manual destruction unless that vault is currently unlocked.

On APFS, copy-on-write filesystems, SSDs, snapshots, and backup systems, software overwriting cannot prove that every historical physical copy disappeared. Destroying the wrapped keys is the primary cryptographic Nuke; the two full overwrite passes are included for the laboratory comparison requested by this project.

## Safety boundary

The wipe implementation refuses to operate unless all of these conditions are true:

- the target is a regular file with a `.nukevol` extension;
- the target has the NukeVault magic header before destruction;
- the target is inside the application's managed vault directory;
- neither the target nor its parent path escapes through a symbolic link; and
- the opened file is the same file that was validated.

It never accepts `/dev/disk*`, `/dev/sd*`, arbitrary paths, directories, or block devices.

## Container design

The current laboratory format is intentionally application-specific:

- Argon2id derives independent keys from the unlock and Nuke passwords.
- A random 256-bit master key is wrapped with the unlock-derived key.
- A separate authenticated token recognizes the Nuke credential.
- AES-256-GCM encrypts and authenticates the vault snapshot.
- Names, directories, metadata, and file contents exist only inside the encrypted payload while locked.
- A random, per-session URL capability protects the loopback WebDAV endpoint.

The decrypted filesystem is kept in memory. Successful WebDAV mutations are encrypted back into the container after each request. WebDAV requests may run concurrently so macOS Finder can complete the parallel metadata and capability requests it issues while streaming an external file upload; encrypted snapshot writes remain serialized. This first version limits a vault to 256 MiB and an individual WebDAV upload to 64 MiB.

This custom format stops generic disk-image tools from mounting a locked container. It does not claim to resist an administrator who can instrument the application, inspect its process memory while unlocked, or reverse engineer the format.

The 10-second rule is enforced only while the application process is alive. Force-quitting the process, cutting power, restoring a snapshot, or copying the container before a challenge can bypass or replay the laboratory scenario. While unlocked, another process running as the same user may also be able to observe or access the local mount. NukeVault is a teaching tool, not an anti-forensics guarantee.

## Platform behavior

### macOS

NukeVault invokes the built-in `mount_webdav` command and mounts inside its private application data directory. Every unlock uses a unique mount-point suffix, so a WebDAV mount left behind by a crashed older process cannot capture the new session. No kernel extension, macFUSE, or FUSE-T installation is required. If a management profile blocks WebDAV mounting, the local session URL remains available and the UI reports the fallback.

Vaults are stored under:

```text
~/Library/Application Support/NukeVault/vaults/
```

### Linux

NukeVault uses `gio mount` when `gio` is installed. This normally exposes the WebDAV volume through the desktop's GVfs integration without FUSE. If `gio` is unavailable or blocked, the app reports the loopback WebDAV URL.

Vaults are normally stored under:

```text
~/.config/NukeVault/vaults/
```

## Development

This directory is its own Go module. Run Go commands here rather than from the monorepo root.

Requirements:

- Go 1.26 or newer;
- Wails 3.0.0;
- Node.js and npm; and
- the platform build dependencies reported by `wails3 doctor`.

Install dependencies and run in development mode:

```sh
cd /Users/tess/github/binarios-go/nuke-program
go mod download
cd frontend && npm install && cd ..
wails3 dev
```

Build the desktop application:

```sh
wails3 build
```

The macOS executable is produced at `bin/nukevault`. Wails packaging tasks can create the final signed application bundle and installer when signing credentials are configured.

## Validation

The tests use only temporary directories and deliberately disposable containers:

```sh
GOCACHE=/tmp/nukevault-gocache go test ./...
```

The suite verifies:

- correct-password unlock and persistence;
- rejection of an ordinary incorrect password;
- immediate Nuke-password detection;
- Nuke after the third incorrect attempt;
- Nuke after the challenge timeout;
- encrypted persistence of WebDAV writes; and
- replacement of an existing file through WebDAV `MOVE`, including Finder-style destination URLs;
- a streamed `PUT` that waits for a parallel `OPTIONS` response, matching Finder's external-upload behavior;
- writing and replacing a file through a real macOS WebDAV mount; and
- refusal to wipe a `.nukevol` file outside the managed directory.

The test that starts WebDAV needs permission to bind to `127.0.0.1`. Some sandboxes block loopback listeners even though no external network connection is made.
