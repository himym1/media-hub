#!/usr/bin/env swift

import AppKit
import CoreGraphics
import Foundation

let arguments = CommandLine.arguments
guard arguments.count == 6 else {
    fputs("usage: render-launcher-icons.swift <1024px-mark-on-white.png> <android-res-dir> <background-hex> <primary-hex> <secondary-hex>\n", stderr)
    exit(2)
}

struct RGB {
    let red: Double
    let green: Double
    let blue: Double
}

func parseHex(_ value: String) -> RGB {
    let hex = value.trimmingCharacters(in: CharacterSet(charactersIn: "#"))
    guard hex.count == 6, let integer = Int(hex, radix: 16) else {
        fputs("invalid launcher palette color: \(value)\n", stderr)
        exit(2)
    }
    return RGB(
        red: Double((integer >> 16) & 0xff),
        green: Double((integer >> 8) & 0xff),
        blue: Double(integer & 0xff)
    )
}

let sourceURL = URL(fileURLWithPath: arguments[1])
let resourceRoot = URL(fileURLWithPath: arguments[2], isDirectory: true)
let backgroundRGB = parseHex(arguments[3])
let markColors = [parseHex(arguments[4]), parseHex(arguments[5])]
guard let sourceImage = NSImage(contentsOf: sourceURL),
      let renderedSource = sourceImage.cgImage(forProposedRect: nil, context: nil, hints: nil) else {
    fputs("failed to read launcher mark\n", stderr)
    exit(1)
}

struct Density {
    let name: String
    let legacy: Int
    let adaptive: Int
}

let densities = [
    Density(name: "mdpi", legacy: 48, adaptive: 108),
    Density(name: "hdpi", legacy: 72, adaptive: 162),
    Density(name: "xhdpi", legacy: 96, adaptive: 216),
    Density(name: "xxhdpi", legacy: 144, adaptive: 324),
    Density(name: "xxxhdpi", legacy: 192, adaptive: 432),
]

let colorSpace = CGColorSpace(name: CGColorSpace.sRGB)!
let background = NSColor(
    srgbRed: backgroundRGB.red / 255.0,
    green: backgroundRGB.green / 255.0,
    blue: backgroundRGB.blue / 255.0,
    alpha: 1
).cgColor

func context(size: Int, data: UnsafeMutableRawPointer? = nil) -> CGContext {
    let value = CGContext(
        data: data,
        width: size,
        height: size,
        bitsPerComponent: 8,
        bytesPerRow: size * 4,
        space: colorSpace,
        bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue | CGBitmapInfo.byteOrder32Big.rawValue
    )!
    value.interpolationQuality = .high
    return value
}


func removeRenderedWhiteBackground(_ image: CGImage) -> CGImage {
    let width = image.width
    let height = image.height
    var pixels = [UInt8](repeating: 0, count: width * height * 4)
    return pixels.withUnsafeMutableBytes { bytes in
        let output = context(size: width, data: bytes.baseAddress)
        output.draw(image, in: CGRect(x: 0, y: 0, width: width, height: height))
        let values = bytes.bindMemory(to: UInt8.self)
        for offset in stride(from: 0, to: values.count, by: 4) {
            let observed = RGB(
                red: Double(values[offset]),
                green: Double(values[offset + 1]),
                blue: Double(values[offset + 2])
            )
            var bestColor = markColors[0]
            var bestAlpha = 0.0
            var bestError = Double.greatestFiniteMagnitude
            for color in markColors {
                let components = [(observed.red, color.red), (observed.green, color.green), (observed.blue, color.blue)]
                let estimates = components.map { (255.0 - $0.0) / max(1.0, 255.0 - $0.1) }
                let alpha = min(1.0, max(0.0, estimates.reduce(0, +) / Double(estimates.count)))
                let predicted = RGB(
                    red: alpha * color.red + (1 - alpha) * 255,
                    green: alpha * color.green + (1 - alpha) * 255,
                    blue: alpha * color.blue + (1 - alpha) * 255
                )
                let error = pow(predicted.red - observed.red, 2) + pow(predicted.green - observed.green, 2) + pow(predicted.blue - observed.blue, 2)
                if error < bestError {
                    bestColor = color
                    bestAlpha = alpha
                    bestError = error
                }
            }
            values[offset] = UInt8((bestColor.red * bestAlpha).rounded())
            values[offset + 1] = UInt8((bestColor.green * bestAlpha).rounded())
            values[offset + 2] = UInt8((bestColor.blue * bestAlpha).rounded())
            values[offset + 3] = UInt8((bestAlpha * 255).rounded())
        }
        return output.makeImage()!
    }
}

let source = removeRenderedWhiteBackground(renderedSource)

func write(_ image: CGImage, to url: URL) throws {
    let representation = NSBitmapImageRep(cgImage: image)
    guard let data = representation.representation(using: .png, properties: [:]) else {
        throw NSError(domain: "MediaHubIcon", code: 1)
    }
    try data.write(to: url, options: .atomic)
}

func render(size: Int, backgroundMode: Bool, round: Bool, monochrome: Bool) -> CGImage {
    let output = context(size: size)
    let rect = CGRect(x: 0, y: 0, width: size, height: size)
    if backgroundMode {
        output.saveGState()
        if round {
            output.addEllipse(in: rect)
            output.clip()
        }
        output.setFillColor(background)
        output.fill(rect)
        output.draw(source, in: rect)
        output.restoreGState()
    } else {
        output.draw(source, in: rect)
        if monochrome {
            output.setBlendMode(.sourceIn)
            output.setFillColor(NSColor.white.cgColor)
            output.fill(rect)
        }
    }
    return output.makeImage()!
}

for density in densities {
    let directory = resourceRoot.appendingPathComponent("mipmap-\(density.name)", isDirectory: true)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    try write(render(size: density.legacy, backgroundMode: true, round: false, monochrome: false), to: directory.appendingPathComponent("ic_launcher.png"))
    try write(render(size: density.legacy, backgroundMode: true, round: true, monochrome: false), to: directory.appendingPathComponent("ic_launcher_round.png"))
    try write(render(size: density.adaptive, backgroundMode: false, round: false, monochrome: false), to: directory.appendingPathComponent("ic_launcher_foreground.png"))
    try write(render(size: density.adaptive, backgroundMode: false, round: false, monochrome: true), to: directory.appendingPathComponent("ic_launcher_monochrome.png"))
}
