import AppKit
import ServiceManagement

/// Menu bar icon: left-click toggles the effect, right-click opens the options menu.
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

    /// Menu bar icon styles: SF Symbol shown while enabled / while disabled.
    private static let iconChoices: [(name: String, on: String, off: String)] = [
        ("Half Circle", "circle.lefthalf.filled", "circle"),
        ("Moon", "moon.fill", "moon"),
        ("Eye", "eye.slash.fill", "eye"),
        ("Brightness", "sun.min.fill", "sun.max"),
        ("Haze", "aqi.medium", "aqi.low"),
        ("Drop", "drop.fill", "drop"),
        ("Layers", "square.2.layers.3d.top.filled", "square.2.layers.3d"),
        ("Window", "macwindow.on.rectangle", "macwindow"),
        ("Lock", "lock.fill", "lock.open"),
    ]

    private var dimmer: Dimmer!
    private var statusItem: NSStatusItem!
    private let menu = NSMenu()
    private let iconMenu = NSMenu()
    private let toggleItem = NSMenuItem(title: "Enabled", action: #selector(toggle), keyEquivalent: "1")
    private let dimItem = NSMenuItem(title: "Dim Background Windows", action: #selector(toggleDim), keyEquivalent: "")
    private let blurItem = NSMenuItem(title: "Blur Background Windows", action: #selector(toggleBlur), keyEquivalent: "")
    private let loginItem = NSMenuItem(title: "Launch at Login", action: #selector(toggleLaunchAtLogin), keyEquivalent: "")

    private var iconIndex: Int {
        let index = UserDefaults.standard.integer(forKey: "icon")
        return Self.iconChoices.indices.contains(index) ? index : 0
    }

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

        for (index, choice) in Self.iconChoices.enumerated() {
            let item = NSMenuItem(title: choice.name, action: #selector(iconChosen(_:)), keyEquivalent: "")
            item.target = self
            item.tag = index
            item.image = NSImage(systemSymbolName: choice.on, accessibilityDescription: choice.name)
            iconMenu.addItem(item)
        }
        let iconItem = NSMenuItem(title: "Menu Bar Icon", action: nil, keyEquivalent: "")
        iconItem.submenu = iconMenu

        menu.addItem(toggleItem)
        menu.addItem(.separator())
        menu.addItem(dimItem)
        menu.addItem(blurItem)
        menu.addItem(.separator())
        menu.addItem(makeLabelItem("Dim Intensity"))
        menu.addItem(makeSliderItem(value: dimmer.intensity, range: 0.1...0.9, action: #selector(intensityChanged(_:))))
        menu.addItem(makeLabelItem("Blur Intensity"))
        menu.addItem(makeSliderItem(value: dimmer.blurIntensity, range: 0.1...1.0, action: #selector(blurIntensityChanged(_:))))
        menu.addItem(.separator())
        menu.addItem(iconItem)
        menu.addItem(loginItem)
        menu.addItem(.separator())
        menu.addItem(withTitle: "Quit Blurzel", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        menu.delegate = self
    }

    private func makeLabelItem(_ title: String) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: nil, keyEquivalent: "")
        item.isEnabled = false
        return item
    }

    private func makeSliderItem(value: Double, range: ClosedRange<Double>, action: Selector) -> NSMenuItem {
        let slider = NSSlider(value: value, minValue: range.lowerBound, maxValue: range.upperBound, target: self, action: action)
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
        for item in iconMenu.items {
            item.state = item.tag == iconIndex ? .on : .off
        }
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

    @objc private func blurIntensityChanged(_ sender: NSSlider) {
        dimmer.blurIntensity = sender.doubleValue
    }

    @objc private func iconChosen(_ sender: NSMenuItem) {
        UserDefaults.standard.set(sender.tag, forKey: "icon")
        updateIcon()
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
        let choice = Self.iconChoices[iconIndex]
        let symbol = dimmer.isEnabled ? choice.on : choice.off
        statusItem.button?.image = NSImage(systemSymbolName: symbol, accessibilityDescription: "Blurzel")
    }
}
