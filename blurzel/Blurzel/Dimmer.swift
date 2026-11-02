import AppKit

/// Dims everything behind the active app by keeping a translucent overlay on
/// each screen, ordered directly below the active app's front windows.
@MainActor
final class Dimmer: NSObject {
    private let ownPID = ProcessInfo.processInfo.processIdentifier
    private var overlays: [NSWindow] = []
    private var timer: Timer?

    var isEnabled: Bool {
        didSet {
            guard isEnabled != oldValue else { return }
            UserDefaults.standard.set(isEnabled, forKey: "enabled")
            isEnabled ? start() : stop()
        }
    }

    var intensity: Double {
        didSet {
            UserDefaults.standard.set(intensity, forKey: "intensity")
            overlays.forEach { $0.backgroundColor = overlayColor }
        }
    }

    private var overlayColor: NSColor { .black.withAlphaComponent(intensity) }

    override init() {
        UserDefaults.standard.register(defaults: ["enabled": true, "intensity": 0.5])
        isEnabled = UserDefaults.standard.bool(forKey: "enabled")
        intensity = UserDefaults.standard.double(forKey: "intensity")
        super.init()
        if isEnabled { start() }
    }

    private func start() {
        let workspace = NSWorkspace.shared.notificationCenter
        workspace.addObserver(self, selector: #selector(refresh), name: NSWorkspace.didActivateApplicationNotification, object: nil)
        workspace.addObserver(self, selector: #selector(refresh), name: NSWorkspace.activeSpaceDidChangeNotification, object: nil)
        NotificationCenter.default.addObserver(self, selector: #selector(rebuildOverlays), name: NSApplication.didChangeScreenParametersNotification, object: nil)
        // Polling catches window switches inside the same app, which post no notification.
        timer = Timer.scheduledTimer(timeInterval: 0.15, target: self, selector: #selector(refresh), userInfo: nil, repeats: true)
        rebuildOverlays()
    }

    private func stop() {
        timer?.invalidate()
        timer = nil
        NSWorkspace.shared.notificationCenter.removeObserver(self)
        NotificationCenter.default.removeObserver(self)
        overlays.forEach { $0.orderOut(nil) }
        overlays = []
    }

    @objc private func rebuildOverlays() {
        overlays.forEach { $0.orderOut(nil) }
        overlays = NSScreen.screens.map(makeOverlay)
        refresh()
    }

    private func makeOverlay(for screen: NSScreen) -> NSWindow {
        let window = NSWindow(contentRect: screen.frame, styleMask: .borderless, backing: .buffered, defer: false)
        window.setFrame(screen.frame, display: false)
        window.backgroundColor = overlayColor
        window.isOpaque = false
        window.hasShadow = false
        window.ignoresMouseEvents = true
        window.isReleasedWhenClosed = false
        window.animationBehavior = .none
        window.level = .normal
        window.collectionBehavior = [.canJoinAllSpaces, .stationary, .ignoresCycle, .fullScreenAuxiliary]
        return window
    }

    @objc private func refresh() {
        guard let frontPID = NSWorkspace.shared.frontmostApplication?.processIdentifier,
              frontPID != ownPID else { return }

        // Normal-level, visible windows, front to back.
        let options: CGWindowListOption = [.optionOnScreenOnly, .excludeDesktopElements]
        let windows = (CGWindowListCopyWindowInfo(options, kCGNullWindowID) as? [[String: Any]] ?? [])
            .filter { ($0[kCGWindowLayer as String] as? Int) == 0 && ($0[kCGWindowAlpha as String] as? Double ?? 1) > 0 }
        let ids = windows.map { $0[kCGWindowNumber as String] as? Int ?? 0 }
        let pids = windows.map { $0[kCGWindowOwnerPID as String] as? pid_t ?? 0 }

        // The front app's topmost run of windows stays bright; everything below it is dimmed.
        guard var anchor = pids.firstIndex(of: frontPID) else {
            overlays.forEach { $0.orderOut(nil) }
            return
        }
        while anchor + 1 < pids.count, pids[anchor + 1] == frontPID { anchor += 1 }

        let overlayIDs = Set(overlays.map(\.windowNumber))
        let belowAnchor = Set(ids.dropFirst(anchor + 1).prefix(overlays.count))
        guard belowAnchor != overlayIDs else { return }

        overlays.forEach { $0.order(.below, relativeTo: ids[anchor]) }
    }
}
