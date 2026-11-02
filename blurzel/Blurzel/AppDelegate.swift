import AppKit
import ServiceManagement

/// Menu bar icon: left-click toggles dimming, right-click opens the options menu.
@main
@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate {
    static func main() {
        let app = NSApplication.shared
        let delegate = AppDelegate()
        app.delegate = delegate
        app.setActivationPolicy(.accessory)
        withExtendedLifetime(delegate) { app.run() }
    }

    private var dimmer: Dimmer!
    private var statusItem: NSStatusItem!
    private let menu = NSMenu()
    private let toggleItem = NSMenuItem(title: "Enabled", action: #selector(toggle), keyEquivalent: "1")
    private let dimItem = NSMenuItem(title: "Dim Background Windows", action: #selector(toggleDim), keyEquivalent: "")
    private let blurItem = NSMenuItem(title: "Blur Background Windows", action: #selector(toggleBlur), keyEquivalent: "")
    private let loginItem = NSMenuItem(title: "Launch at Login", action: #selector(toggleLaunchAtLogin), keyEquivalent: "")

    func applicationDidFinishLaunching(_ notification: Notification) {
        dimmer = Dimmer()

        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        if let button = statusItem.button {
            button.target = self
            button.action = #selector(statusItemClicked)
            button.sendAction(on: [.leftMouseUp, .rightMouseUp])
            button.toolTip = "Blurzel — click or ⇧⌘1 to toggle, right-click for options"
        }

        HotKey.register()
        NotificationCenter.default.addObserver(self, selector: #selector(toggle), name: .hotKeyPressed, object: nil)

        buildMenu()
        updateIcon()
    }

    private func buildMenu() {
        toggleItem.target = self
        toggleItem.keyEquivalentModifierMask = [.command, .shift]
        dimItem.target = self
        blurItem.target = self
        loginItem.target = self

        let intensityLabel = NSMenuItem(title: "Intensity", action: nil, keyEquivalent: "")
        intensityLabel.isEnabled = false

        menu.addItem(toggleItem)
        menu.addItem(.separator())
        menu.addItem(dimItem)
        menu.addItem(blurItem)
        menu.addItem(.separator())
        menu.addItem(intensityLabel)
        menu.addItem(makeSliderItem())
        menu.addItem(.separator())
        menu.addItem(loginItem)
        menu.addItem(.separator())
        menu.addItem(withTitle: "Quit Blurzel", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        menu.delegate = self
    }

    private func makeSliderItem() -> NSMenuItem {
        let slider = NSSlider(value: dimmer.intensity, minValue: 0.1, maxValue: 0.9, target: self, action: #selector(intensityChanged(_:)))
        slider.frame = NSRect(x: 20, y: 2, width: 180, height: 24)
        let container = NSView(frame: NSRect(x: 0, y: 0, width: 220, height: 28))
        container.addSubview(slider)
        let item = NSMenuItem()
        item.view = container
        return item
    }

    func menuNeedsUpdate(_ menu: NSMenu) {
        toggleItem.state = dimmer.isEnabled ? .on : .off
        dimItem.state = dimmer.isDimEnabled ? .on : .off
        blurItem.state = dimmer.isBlurEnabled ? .on : .off
        loginItem.state = SMAppService.mainApp.status == .enabled ? .on : .off
    }

    @objc private func statusItemClicked() {
        let event = NSApp.currentEvent
        if event?.type == .rightMouseUp || event?.modifierFlags.contains(.control) == true {
            statusItem.menu = menu
            statusItem.button?.performClick(nil)
            statusItem.menu = nil
        } else {
            toggle()
        }
    }

    @objc private func toggle() {
        dimmer.isEnabled.toggle()
        updateIcon()
    }

    @objc private func toggleDim() {
        dimmer.isDimEnabled.toggle()
    }

    @objc private func toggleBlur() {
        dimmer.isBlurEnabled.toggle()
    }

    @objc private func intensityChanged(_ sender: NSSlider) {
        dimmer.intensity = sender.doubleValue
    }

    @objc private func toggleLaunchAtLogin() {
        let service = SMAppService.mainApp
        do {
            if service.status == .enabled {
                try service.unregister()
            } else {
                try service.register()
            }
        } catch {
            NSLog("Blurzel: launch at login failed: \(error)")
        }
    }

    private func updateIcon() {
        let symbol = dimmer.isEnabled ? "circle.lefthalf.filled" : "circle"
        statusItem.button?.image = NSImage(systemSymbolName: symbol, accessibilityDescription: "Blurzel")
    }
}
