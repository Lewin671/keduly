import SwiftUI
import UIKit

/// The colours of the design, light and dark. Orange means "important" and nothing else; red is
/// for deadlines, now and today; tomato red only for tomato marks.
enum Palette {
    private static func dynamic(_ light: UInt32, _ dark: UInt32) -> UIColor {
        UIColor { $0.userInterfaceStyle == .dark ? UIColor(hex: dark) : UIColor(hex: light) }
    }

    static let tintUI = dynamic(0x2867D6, 0x6C9DFF)
    static let redUI = dynamic(0xFF3B30, 0xFF453A)
    static let orangeUI = dynamic(0xF08C00, 0xFF9F0A)
    static let greenUI = dynamic(0x34C759, 0x30D158)
    static let blueUI = dynamic(0x007AFF, 0x0A84FF)
    static let indigoUI = dynamic(0x5856D6, 0x7D7AFF)
    static let purpleUI = dynamic(0xAF52DE, 0xBF5AF2)
    static let tealUI = dynamic(0x30B0C7, 0x40C8E0)
    static let pinkUI = dynamic(0xFF2D55, 0xFF375F)
    static let brownUI = dynamic(0xA2845E, 0xAC8E68)
    static let tomatoUI = dynamic(0xF25C45, 0xFF6B55)

    static let tint = Color(uiColor: tintUI)
    static let red = Color(uiColor: redUI)
    static let orange = Color(uiColor: orangeUI)
    static let green = Color(uiColor: greenUI)
    static let teal = Color(uiColor: tealUI)
    static let blue = Color(uiColor: blueUI)
    static let tomato = Color(uiColor: tomatoUI)
    static let star = Color(uiColor: UIColor(hex: 0xF5B400))
    /// Text and icons on a tint-filled button.
    static let onTint = Color(uiColor: dynamic(0xFFFFFF, 0x06122B))

    static let label = Color(uiColor: .label)
    static let label2 = Color(uiColor: .secondaryLabel)
    static let label3 = Color(uiColor: .tertiaryLabel)
    static let separator = Color(uiColor: .separator)
    static let fill = Color(uiColor: .tertiarySystemFill)
    static let fill2 = Color(uiColor: .secondarySystemFill)
    static let background = Color(uiColor: .systemBackground)
    static let grouped = Color(uiColor: .systemGroupedBackground)
    static let cell = Color(uiColor: .secondarySystemGroupedBackground)

    static func projectUI(_ color: ProjectColor?) -> UIColor {
        switch color {
        case .blue: return blueUI
        case .indigo: return indigoUI
        case .orange: return orangeUI
        case .teal: return tealUI
        case .green: return greenUI
        case .pink: return pinkUI
        case .purple: return purpleUI
        case .brown: return brownUI
        case nil: return tintUI
        }
    }

    /// A project's colour; the brand tint for things without a project.
    static func project(_ color: ProjectColor?) -> Color {
        Color(uiColor: projectUI(color))
    }

    /// `fraction` of the colour over the page background: the fill of a calendar block.
    static func wash(_ color: UIColor, _ fraction: CGFloat) -> Color {
        Color(uiColor: UIColor { traits in
            color.resolvedColor(with: traits).mixed(fraction, with: UIColor.systemBackground.resolvedColor(with: traits))
        })
    }

    /// The colour darkened in light mode and lightened in dark mode: the text on a calendar block.
    static func ink(_ color: UIColor, _ fraction: CGFloat = 0.64) -> Color {
        Color(uiColor: UIColor { traits in
            color.resolvedColor(with: traits).mixed(fraction, with: traits.userInterfaceStyle == .dark ? .white : .black)
        })
    }
}

extension UIColor {
    convenience init(hex: UInt32) {
        self.init(red: CGFloat((hex >> 16) & 0xFF) / 255, green: CGFloat((hex >> 8) & 0xFF) / 255, blue: CGFloat(hex & 0xFF) / 255, alpha: 1)
    }

    /// `fraction` of this colour and the rest of the other.
    func mixed(_ fraction: CGFloat, with other: UIColor) -> UIColor {
        var r1: CGFloat = 0, g1: CGFloat = 0, b1: CGFloat = 0, a1: CGFloat = 0
        var r2: CGFloat = 0, g2: CGFloat = 0, b2: CGFloat = 0, a2: CGFloat = 0
        getRed(&r1, green: &g1, blue: &b1, alpha: &a1)
        other.getRed(&r2, green: &g2, blue: &b2, alpha: &a2)
        let k = 1 - fraction
        return UIColor(red: r1 * fraction + r2 * k, green: g1 * fraction + g2 * k, blue: b1 * fraction + b2 * k, alpha: 1)
    }
}

extension Font {
    /// The system font at one of the design's sizes, following Dynamic Type.
    ///
    /// Each size the design uses is the default size of a system text style, so text looks the
    /// same at the default setting and grows with the user's. A size that belongs to no style
    /// stays fixed: those are the places that cannot grow (the dial, the tab bar, the day numbers).
    static func app(_ size: CGFloat, _ weight: Font.Weight = .regular) -> Font {
        guard let style = AppType.style(for: size) else { return .system(size: size, weight: weight) }
        return .system(style).weight(weight)
    }
}

enum AppType {
    static func style(for size: CGFloat) -> Font.TextStyle? {
        switch size {
        case 34: return .largeTitle
        case 28: return .title
        case 22: return .title2
        case 20: return .title3
        case 17: return .body
        case 16: return .callout
        case 15: return .subheadline
        case 13: return .footnote
        case 12: return .caption
        case 11, 11.5: return .caption2
        default: return nil
        }
    }

    /// The largest setting for parts whose frame is fixed: the tab bar, the calendar grid, the time axis, the chart.
    static let fixedLayoutLimit = DynamicTypeSize.xLarge
}

/// The height the floating tab bar takes at the bottom of a page.
enum Metrics {
    static let tabBarHeight: CGFloat = 62
    static let pageMargin: CGFloat = 20
}
