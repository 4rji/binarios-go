import AppKit
import CoreGraphics

/// Owns one private capture directory and at most one screencapture process.
@MainActor
final class ScreenCapture {
    private let executableURL: URL
    private let temporaryRoot: URL
    private let pasteboard: NSPasteboard
    private let preflightAccess: () -> Bool
    private let requestAccess: () -> Bool
    private var process: Process?
    private var directory: URL?
    private var clipboardGuard: CaptureClipboardGuard?

    // Internal injection points keep capture/error tests independent of TCC and
    // the user's screen. Production always uses the native utility and APIs.
    init(executableURL: URL = URL(fileURLWithPath: "/usr/sbin/screencapture"),
         temporaryRoot: URL = FileManager.default.temporaryDirectory,
         pasteboard: NSPasteboard = .general,
         preflightAccess: @escaping () -> Bool = { CGPreflightScreenCaptureAccess() },
         requestAccess: @escaping () -> Bool = { CGRequestScreenCaptureAccess() }) {
        self.executableURL = executableURL
        self.temporaryRoot = temporaryRoot
        self.pasteboard = pasteboard
        self.preflightAccess = preflightAccess
        self.requestAccess = requestAccess
    }

    func capture() async throws -> URL {
        guard preflightAccess() else {
            let granted = requestAccess()
            guard granted else {
                throw MacOCRError.message("Screen Recording permission is missing. Enable the launching terminal or macocr in System Settings > Privacy & Security > Screen Recording (or Screen & System Audio Recording), then restart it. Run interactively once before using a LaunchAgent.")
            }
            // Some macOS versions require a relaunch even after accepting the prompt.
            guard preflightAccess() else {
                throw MacOCRError.message("Screen Recording was requested. Restart the launching terminal or MacOCR after granting permission in System Settings.")
            }
            return try await runCapture()
        }
        return try await runCapture()
    }

    private func runCapture() async throws -> URL {
        let folder = temporaryRoot.appendingPathComponent("MacOCR-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: false,
                                               attributes: [.posixPermissions: 0o700])
        directory = folder
        let imageURL = folder.appendingPathComponent("region.png")
        let diagnosticsURL = folder.appendingPathComponent("capture-errors.log")
        guard FileManager.default.createFile(atPath: diagnosticsURL.path, contents: nil,
                                             attributes: [.posixPermissions: 0o600]) else {
            cleanup()
            throw MacOCRError.message("Could not create the private capture diagnostics file.")
        }
        let errors: FileHandle
        do { errors = try FileHandle(forWritingTo: diagnosticsURL) }
        catch { cleanup(); throw error }
        let child = Process()
        child.executableURL = executableURL
        // -s enforces a region selection; -x silences the capture sound.
        child.arguments = ["-i", "-s", "-x", "-t", "png", imageURL.path]
        child.standardOutput = FileHandle.nullDevice
        child.standardError = errors
        process = child
        clipboardGuard = CaptureClipboardGuard(board: pasteboard)

        return try await withCheckedThrowingContinuation { continuation in
            child.terminationHandler = { [weak self] completed in
                let status = completed.terminationStatus
                // A native helper may inherit stderr and outlive screencapture.
                // Never wait for pipe EOF here: that would prevent continuation
                // resumption, OCR, clipboard delivery, and the next hotkey.
                try? errors.close()
                let diagnostics = (try? String(contentsOf: diagnosticsURL, encoding: .utf8))?
                    .trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
                Task { @MainActor [weak self] in
                    self?.process = nil
                    let exists = FileManager.default.fileExists(atPath: imageURL.path)
                    do {
                        if let self {
                            try self.clipboardGuard?.restoreCaptureSideEffect(board: self.pasteboard, fileWasCreated: exists)
                        }
                        self?.clipboardGuard = nil
                    } catch {
                        self?.cleanup()
                        continuation.resume(throwing: error)
                        return
                    }
                    if status == 0 && exists {
                        continuation.resume(returning: imageURL)
                    } else {
                        self?.cleanup()
                        if let self, !self.preflightAccess() {
                            continuation.resume(throwing: MacOCRError.message("Screen Recording permission was denied or revoked. Grant it in System Settings and restart the launching process."))
                        } else if !exists && (status == 0 || status == 1) && diagnostics.isEmpty {
                            continuation.resume(throwing: MacOCRError.cancelled)
                        } else {
                            continuation.resume(throwing: MacOCRError.message("Screen capture failed (exit \(status))\(diagnostics.isEmpty ? "." : ": \(diagnostics)")"))
                        }
                    }
                }
            }
            do { try child.run() }
            catch {
                child.terminationHandler = nil
                try? errors.close()
                process = nil
                clipboardGuard = nil
                cleanup()
                continuation.resume(throwing: MacOCRError.message("Could not start screencapture: \(error.localizedDescription)"))
            }
        }
    }

    func cleanup() {
        if let directory {
            do {
                try FileManager.default.removeItem(at: directory)
                self.directory = nil
            } catch {
                FileHandle.standardError.write(Data("macocr: Could not remove temporary capture: \(error.localizedDescription)\n".utf8))
            }
        }
    }

    func stop() {
        if let process, process.isRunning {
            process.terminate()
            // The capture child must exit before removal, so it cannot recreate the file.
            process.waitUntilExit()
        }
        process = nil
        let fileExists = directory.map { FileManager.default.fileExists(atPath: $0.appendingPathComponent("region.png").path) } ?? false
        do { try clipboardGuard?.restoreCaptureSideEffect(board: pasteboard, fileWasCreated: fileExists) }
        catch { FileHandle.standardError.write(Data("macocr: \(error.localizedDescription)\n".utf8)) }
        clipboardGuard = nil
        cleanup()
    }
}
