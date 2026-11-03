import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

// Preserve the artwork while matching the visual footprint of macOS Dock icons.
guard CommandLine.arguments.count == 3 else {
    fatalError("Usage: round-icon.swift input.png output.png")
}
let input = URL(fileURLWithPath: CommandLine.arguments[1])
let output = URL(fileURLWithPath: CommandLine.arguments[2])
guard let source = CGImageSourceCreateWithURL(input as CFURL, nil),
      let image = CGImageSourceCreateImageAtIndex(source, 0, nil),
      let context = CGContext(data: nil, width: 1024, height: 1024,
                              bitsPerComponent: 8, bytesPerRow: 0,
                              space: CGColorSpaceCreateDeviceRGB(),
                              bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else {
    fatalError("Cannot load icon artwork")
}
let canvas = CGRect(x: 0, y: 0, width: 1024, height: 1024)
let bounds = canvas.insetBy(dx: 88, dy: 88)
let cornerRadius = 224 * bounds.width / canvas.width
context.clear(canvas)
context.interpolationQuality = .high
context.addPath(CGPath(roundedRect: bounds, cornerWidth: cornerRadius, cornerHeight: cornerRadius, transform: nil))
context.clip()
context.draw(image, in: bounds)
guard let result = context.makeImage(),
      let destination = CGImageDestinationCreateWithURL(output as CFURL, UTType.png.identifier as CFString, 1, nil) else {
    fatalError("Cannot create rounded icon")
}
CGImageDestinationAddImage(destination, result, nil)
guard CGImageDestinationFinalize(destination) else {
    fatalError("Cannot save rounded icon")
}
