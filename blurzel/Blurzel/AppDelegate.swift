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

        updateIcon()
    }

    /// Built fresh on every open, so a control left mid-drag can't stay stuck.
    private func makeMenu() -> NSMenu {
        let toggleItem = makeItem("Enabled", #selector(toggle), isOn: dimmer.isEnabled)
        toggleItem.keyEquivalent = "1"
        toggleItem.keyEquivalentModifierMask = [.command, .shift]

        let iconMenu = NSMenu()
        for (index, choice) in Self.iconChoices.enumerated() {
            let item = makeItem(choice.name, #selector(iconChosen(_:)), isOn: index == iconIndex)
            item.tag = index
            item.image = NSImage(systemSymbolName: choice.on, accessibilityDescription: choice.name)
            iconMenu.addItem(item)
        }
        let iconItem = NSMenuItem(title: "Menu Bar Icon", action: nil, keyEquivalent: "")
        iconItem.submenu = iconMenu

        let menu = NSMenu()
        menu.delegate = self
        menu.addItem(toggleItem)
        menu.addItem(.separator())
        menu.addItem(makeItem("Dim Background Windows", #selector(toggleDim), isOn: dimmer.isDimEnabled))
        menu.addItem(makeItem("Blur Background Windows", #selector(toggleBlur), isOn: dimmer.isBlurEnabled))
        menu.addItem(.separator())
        menu.addItem(makeStepsItem("Dim Intensity", value: dimmer.intensity, percents: Array(stride(from: 10, through: 90, by: 10)), action: #selector(intensityChosen(_:))))
        menu.addItem(makeStepsItem("Blur Intensity", value: dimmer.blurIntensity, percents: Array(stride(from: 10, through: 100, by: 10)), action: #selector(blurIntensityChosen(_:))))
        menu.addItem(.separator())
        menu.addItem(iconItem)
        menu.addItem(makeItem("Launch at Login", #selector(toggleLaunchAtLogin), isOn: SMAppService.mainApp.status == .enabled))
        menu.addItem(.separator())
        menu.addItem(withTitle: "Quit Blurzel", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        return menu
    }

    private func makeItem(_ title: String, _ action: Selector, isOn: Bool) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: "")
        item.target = self
        item.state = isOn ? .on : .off
        return item
    }

    /// A submenu of percentage steps. Plain items instead of an NSSlider view, which can
    /// freeze while dragged inside a menu on recent macOS.
    private func makeStepsItem(_ title: String, value: Double, percents: [Int], action: Selector) -> NSMenuItem {
        let current = Int((value * 100).rounded())
        let nearest = percents.min { abs($0 - current) < abs($1 - current) }
        let submenu = NSMenu()
        for percent in percents {
            let item = makeItem("\(percent)%", action, isOn: percent == nearest)
            item.tag = percent
            submenu.addItem(item)
        }
        let item = NSMenuItem(title: "\(title) (\(current)%)", action: nil, keyEquivalent: "")
        item.submenu = submenu
        return item
    }

    @objc private func statusItemClicked() {
        let event = NSApp.currentEvent
        if event?.type == .rightMouseUp || event?.modifierFlags.contains(.control) == true {
            statusItem.menu = makeMenu()
            statusItem.button?.performClick(nil)
        } else {
            toggle()
        }
    }

    // Detach only once the menu has really closed, so left-click goes back to toggling.
    func menuDidClose(_ menu: NSMenu) {
        statusItem.menu = nil
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

    @objc private func intensityChosen(_ sender: NSMenuItem) {
        dimmer.intensity = Double(sender.tag) / 100
    }

    @objc private func blurIntensityChosen(_ sender: NSMenuItem) {
        dimmer.blurIntensity = Double(sender.tag) / 100
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
