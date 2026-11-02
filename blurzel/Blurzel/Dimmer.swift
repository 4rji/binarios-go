import AppKit

/// Dims, and optionally blurs, everything behind the active app by keeping an
/// overlay on each screen, ordered directly below the active app's front windows.
@MainActor
final class Dimmer: NSObject {
    private let ownPID = ProcessInfo.processInfo.processIdentifier
    private var overlays: [Overlay] = []
    private var timer: Timer?

    var isEnabled: Bool {
        didSet {
            guard isEnabled != oldValue else { return }
            UserDefaults.standard.set(isEnabled, forKey: "enabled")
            isEnabled ? start() : stop()
        }
    }

    var isDimEnabled: Bool {
        didSet {
            UserDefaults.standard.set(isDimEnabled, forKey: "dim")
            applyStyle()
        }
    }

    var intensity: Double {
        didSet {
            UserDefaults.standard.set(intensity, forKey: "intensity")
            applyStyle()
        }
    }

    var isBlurEnabled: Bool {
        didSet {
            UserDefaults.standard.set(isBlurEnabled, forKey: "blur")
            applyStyle()
        }
    }

    override init() {
        UserDefaults.standard.register(defaults: ["enabled": true, "dim": true, "intensity": 0.5, "blur": false])
        isEnabled = UserDefaults.standard.bool(forKey: "enabled")
        isDimEnabled = UserDefaults.standard.bool(forKey: "dim")
        intensity = UserDefaults.standard.double(forKey: "intensity")
        isBlurEnabled = UserDefaults.standard.bool(forKey: "blur")
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
        overlays.forEach { $0.window.orderOut(nil) }
        overlays = []
    }

    @objc private func rebuildOverlays() {
        overlays.forEach { $0.window.orderOut(nil) }
        overlays = NSScreen.screens.map(Overlay.init(screen:))
        applyStyle()
        refresh()
    }

    private func applyStyle() {
        overlays.forEach { $0.apply(dim: isDimEnabled ? intensity : 0, blur: isBlurEnabled) }
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

        // The front app's topmost run of windows stays clear; everything below it is covered.
        guard var anchor = pids.firstIndex(of: frontPID) else {
            overlays.forEach { $0.window.orderOut(nil) }
            return
        }
        while anchor + 1 < pids.count, pids[anchor + 1] == frontPID { anchor += 1 }

        let overlayIDs = Set(overlays.map(\.window.windowNumber))
        let belowAnchor = Set(ids.dropFirst(anchor + 1).prefix(overlays.count))
        guard belowAnchor != overlayIDs else { return }

        overlays.forEach { $0.window.order(.below, relativeTo: ids[anchor]) }
    }
}

/// A click-through, full-screen window that dims, and optionally blurs, whatever is behind it.
@MainActor
private final class Overlay {
    let window: NSWindow
    private let blurView = NSVisualEffectView()
    private let dimView = NSView()

    init(screen: NSScreen) {
        window = NSWindow(contentRect: screen.frame, styleMask: .borderless, backing: .buffered, defer: false)
        window.setFrame(screen.frame, display: false)
        window.backgroundColor = .clear
        window.isOpaque = false
        window.hasShadow = false
        window.ignoresMouseEvents = true
        window.isReleasedWhenClosed = false
        window.animationBehavior = .none
        window.level = .normal
        window.collectionBehavior = [.canJoinAllSpaces, .stationary, .ignoresCycle, .fullScreenAuxiliary]
        window.appearance = NSAppearance(named: .darkAqua)

        // Behind-window blending blurs the other apps' windows under the overlay.
        blurView.blendingMode = .behindWindow
        blurView.material = .fullScreenUI
        blurView.state = .active
        dimView.wantsLayer = true

        let container = NSView(frame: NSRect(origin: .zero, size: screen.frame.size))
        for view in [blurView, dimView] {
            view.frame = container.bounds
            view.autoresizingMask = [.width, .height]
            container.addSubview(view)
        }
        window.contentView = container
    }

    func apply(dim: Double, blur: Bool) {
        dimView.layer?.backgroundColor = NSColor.black.withAlphaComponent(dim).cgColor
        blurView.isHidden = !blur
    }
}
