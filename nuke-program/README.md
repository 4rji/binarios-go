# NukeVault

NukeVault is a deliberately destructive encrypted vault for controlled digital-forensics labs. It is a Go and Wails 3 desktop application for macOS and Linux. It does not require FUSE: an unlocked vault is exposed through a loopback-only WebDAV server and mounted with the operating system's user-space WebDAV integration when available.

> **Warning:** NukeVault is intentionally capable of irreversible data loss. Use only disposable classroom evidence. Never store the only copy of real data in a `.nukevol` container.

## Destructive challenge

The unlock screen is enforced by the Go backend, not only by JavaScript:

- the normal password unlocks the vault;
- the dedicated Nuke password immediately starts destruction;
- three incorrect passwords start destruction; and
- entering nothing for 10 seconds starts destruction.

After an incorrect attempt, a new 10-second window begins. Attempts and the absolute deadline are atomically persisted per vault. Closing and reopening NukeVault therefore preserves both `1/3` or `2/3` and the existing deadline; it does not grant another set of attempts or another 10 seconds. If the deadline expires while the app is closed, opening it starts the Nuke sequence immediately. A successful unlock clears the persisted challenge.

The Nuke sequence:

1. Immediately closes the active WebDAV session, discards its in-memory key, and force-unmounts the managed volume if necessary.
2. Overwrites the first MiB containing the encrypted key slots and header with cryptographically random data.
3. Overwrites the complete container with random data.
4. Overwrites the complete container with zeros.
5. Flushes writes and deletes the container.

After a successful unlock, the UI also offers **Destroy Vault**. It opens an in-app warning with separate **Cancel** and **Confirm Destroy** buttons before starting the same Nuke sequence. The backend refuses manual destruction unless that vault is currently unlocked. Destruction closes the WebDAV server without saving an unfinished upload, so a stalled Finder copy cannot hold the wipe open.

**Lock & Exit** unmounts the decrypted view, closes the local WebDAV session, clears the in-memory runtime state, and then quits NukeVault. It does not return to the unlock screen or start a new challenge countdown.

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

Before creating a vault, **Choose Folder** can select Documents, iCloud Drive, or another writable directory through the native macOS/Linux folder dialog. NukeVault creates a dedicated `NukeVault/` subdirectory inside the selected parent and remembers that location in its local settings. The wipe remains scoped to the selected dedicated directory and still refuses arbitrary files, directories, and devices.

**Open Existing `.nukevol`** can import a vault created on another machine. NukeVault validates the container and copies it into the current managed vault directory before arming the unlock challenge. The transferred source file remains untouched and is outside the wipe scope; remove that extra copy separately if the laboratory exercise requires a single managed container.

Cloud storage changes the destruction guarantee: iCloud, OneDrive, Dropbox, snapshots, and backup services may retain remote versions or deleted copies after the local `.nukevol` has been cryptographically nuked and overwritten. The persisted failed-attempt counter remains local to this installation, so a second computer does not inherit it reliably through cloud sync.

This custom format stops generic disk-image tools from mounting a locked container. It does not claim to resist an administrator who can instrument the application, inspect its process memory while unlocked, or reverse engineer the format.

The 10-second rule is enforced only while the application process is alive. Force-quitting the process, cutting power, restoring a snapshot, or copying the container before a challenge can bypass or replay the laboratory scenario. While unlocked, another process running as the same user may also be able to observe or access the local mount. NukeVault is a teaching tool, not an anti-forensics guarantee.

## Platform behavior

### macOS

NukeVault invokes the built-in `mount_webdav` command and mounts inside its private application data directory. Every unlock uses a unique mount-point suffix, so a WebDAV mount left behind by a crashed older process cannot capture the new session. No kernel extension, macFUSE, or FUSE-T installation is required. If a management profile blocks WebDAV mounting, the local session URL remains available and the UI reports the fallback.

The native folder selector can choose `Documents`, `iCloud Drive`, or an existing `NukeVault` folder. When a parent such as `Documents` is selected, the resulting container path is `Documents/NukeVault/<id>.nukevol`.

Vaults are stored under:

```text
~/Library/Application Support/NukeVault/vaults/
```

Persistent challenge counters are stored under the adjacent private `challenges/` directory.

### Linux

NukeVault uses `gio mount` when `gio` is installed. This normally exposes the WebDAV volume through the desktop's GVfs integration without FUSE. If `gio` is unavailable or blocked, the app reports the loopback WebDAV URL.

Vaults are normally stored under:

```text
~/.config/NukeVault/vaults/
```

Persistent challenge counters are stored under `~/.config/NukeVault/challenges/`.

## Development

This directory is its own Go module. Run Go commands here rather than from the monorepo root.

Requirements:

- Go 1.26 or newer;
- Wails 3;
- Node.js and npm; and
- the platform build dependencies reported by `wails3 doctor`.

Install the Wails 3 CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

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
- persistence of failed attempts and the absolute deadline across process restarts;
- immediate Nuke when a persisted deadline elapsed while the app was closed;
- encrypted persistence of WebDAV writes; and
- replacement of an existing file through WebDAV `MOVE`, including Finder-style destination URLs;
- a streamed `PUT` that waits for a parallel `OPTIONS` response, matching Finder's external-upload behavior;
- destruction of a session while a streamed `PUT` is deliberately stalled;
- writing and replacing a file through a real macOS WebDAV mount; and
- refusal to wipe a `.nukevol` file outside the managed directory.

The test that starts WebDAV needs permission to bind to `127.0.0.1`. Some sandboxes block loopback listeners even though no external network connection is made.
