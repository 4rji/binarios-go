#!/usr/bin/env swift
import AppKit

// Generic app icon: SF Symbol glyph centered on a rounded-square background.
let size = 1024.0
let sizeConfig = NSImage.SymbolConfiguration(pointSize: size * 0.52, weight: .medium)
let colorConfig = NSImage.SymbolConfiguration(paletteColors: [.white])
guard let symbol = NSImage(systemSymbolName: "text.viewfinder", accessibilityDescription: nil)?
    .withSymbolConfiguration(sizeConfig.applying(colorConfig)) else {
    FileHandle.standardError.write(Data("generate-icon: could not load SF Symbol\n".utf8))
    exit(1)
}

let canvas = NSImage(size: NSSize(width: size, height: size))
canvas.lockFocus()
let rect = NSRect(x: 0, y: 0, width: size, height: size)
let background = NSBezierPath(roundedRect: rect.insetBy(dx: size * 0.04, dy: size * 0.04),
                               xRadius: size * 0.22, yRadius: size * 0.22)
NSColor.systemBlue.setFill()
background.fill()
let origin = NSPoint(x: (size - symbol.size.width) / 2, y: (size - symbol.size.height) / 2)
symbol.draw(at: origin, from: .zero, operation: .sourceOver, fraction: 1.0)
canvas.unlockFocus()

guard let tiff = canvas.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff),
      let png = bitmap.representation(using: .png, properties: [:]) else {
    FileHandle.standardError.write(Data("generate-icon: could not render PNG\n".utf8))
    exit(1)
}
let outputPath = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "icon.png"
try png.write(to: URL(fileURLWithPath: outputPath))
print("Wrote \(outputPath)")
