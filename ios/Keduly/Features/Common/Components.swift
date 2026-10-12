import SwiftUI

/// Runs a load and keeps its last good result. A failure keeps what is on screen.
@MainActor
@Observable
final class Loader<Value: Sendable> {
    var value: Value?
    var failed = false

    func load(_ model: AppModel, _ operation: @Sendable (APIClient) async throws -> Value) async {
        do {
            value = try await operation(model.api)
            failed = false
        } catch is CancellationError {
        } catch APIError.unauthenticated {
        } catch {
            if !Task.isCancelled { failed = true }
        }
    }
}

/// A scrolling page: the large title is part of the content, as in the design.
struct Screen<Content: View>: View {
    var failed = false
    var grouped = false
    var retry: (() async -> Void)?
    @ViewBuilder var content: Content

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 0) {
                if failed, let retry { ErrorLine(retry: retry) }
                content
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, Metrics.pageMargin)
            .padding(.bottom, 24)
        }
        .background(grouped ? Palette.grouped : Palette.background)
        .refreshable { await retry?() }
        .inlineTitle()
    }
}

extension View {
    /// The navigation bar stays empty: pages draw their own large title. The title is still set by
    /// the caller with `navigationTitle` so that the back button of the next page can name it.
    func inlineTitle() -> some View {
        navigationBarTitleDisplayMode(.inline)
            .toolbar { ToolbarItem(placement: .principal) { Color.clear.frame(width: 1, height: 1).accessibilityHidden(true) } }
    }
}

/// "The server cannot be reached", with what is on screen left as it is.
struct ErrorLine: View {
    let retry: () async -> Void
    @State private var busy = false

    var body: some View {
        HStack(spacing: 8) {
            Text(L("error.network")).font(.system(size: 15)).foregroundStyle(Palette.label2)
            Spacer()
            Button(L("common.retry")) {
                busy = true
                Task {
                    await retry()
                    busy = false
                }
            }
            .font(.system(size: 15, weight: .semibold))
            .disabled(busy)
        }
        .padding(.vertical, 10)
        .accessibilityIdentifier("error-line")
    }
}

struct LargeTitle<Leading: View>: View {
    let text: String
    var detail: String?
    var lineLimit = 1
    @ViewBuilder var leading: Leading

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: 10) {
            leading.alignmentGuide(.firstTextBaseline) { $0[.bottom] - 4 }
            Text(text)
                .font(.system(size: 34, weight: .bold))
                .tracking(-0.6)
                .lineLimit(lineLimit)
                .accessibilityAddTraits(.isHeader)
            if let detail {
                Text(detail).font(.system(size: 17)).foregroundStyle(Palette.label2).lineLimit(1)
            }
        }
        .padding(.top, 2)
        .padding(.bottom, 6)
    }
}

extension LargeTitle where Leading == EmptyView {
    init(_ text: String, detail: String? = nil) {
        self.init(text: text, detail: detail) { EmptyView() }
    }
}

/// A section heading inside a list page: a bold name, an optional small note, and a figure on the right.
struct SectionHeader<Icon: View>: View {
    let title: String
    var note: String?
    var trailing: String?
    var color: Color = Palette.label
    @ViewBuilder var icon: Icon

    var body: some View {
        VStack(spacing: 0) {
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                icon
                Text(title).font(.system(size: 20, weight: .bold)).tracking(-0.4).foregroundStyle(color).lineLimit(1)
                if let note { Text(note).font(.system(size: 13)).foregroundStyle(Palette.label2).lineLimit(1) }
                Spacer(minLength: 8)
                if let trailing { Text(trailing).font(.system(size: 13)).foregroundStyle(Palette.label2).lineLimit(1) }
            }
            .padding(.top, 22)
            .padding(.bottom, 7)
            Divider()
        }
        .accessibilityElement(children: .combine)
        .accessibilityAddTraits(.isHeader)
    }
}

extension SectionHeader where Icon == EmptyView {
    init(_ title: String, note: String? = nil, trailing: String? = nil, color: Color = Palette.label) {
        self.init(title: title, note: note, trailing: trailing, color: color) { EmptyView() }
    }
}

/// An empty page: a heading and a line or two. No illustration and no button.
struct Blank: View {
    let title: String
    let text: String
    var top: CGFloat = 130

    var body: some View {
        VStack(spacing: 6) {
            Text(title).font(.system(size: 20, weight: .semibold)).foregroundStyle(Palette.label)
            Text(text).font(.system(size: 15)).foregroundStyle(Palette.label2).multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, top)
        .padding(.horizontal, 6)
    }
}

/// The rounded square that ticks an item. 22 pt drawn, 44 pt to touch.
struct Checkbox: View {
    let on: Bool
    var color: Color = Palette.tint
    var size: CGFloat = 22
    let label: String
    let action: () async -> Void
    @State private var busy = false

    var body: some View {
        Button {
            guard !busy else { return }
            busy = true
            Task {
                await action()
                busy = false
            }
        } label: {
            ZStack {
                RoundedRectangle(cornerRadius: size * 0.32, style: .continuous)
                    .fill(on ? color : .clear)
                RoundedRectangle(cornerRadius: size * 0.32, style: .continuous)
                    .strokeBorder(on ? color : Palette.label3, lineWidth: 1.6)
                if on {
                    Image(systemName: "checkmark").font(.system(size: size * 0.5, weight: .bold)).foregroundStyle(.white)
                }
            }
            .frame(width: size, height: size)
            .frame(width: 44, height: 44)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(busy)
        .accessibilityLabel(label)
        .accessibilityAddTraits(on ? .isSelected : [])
    }
}

/// The tomato mark: filled in tomato red when earned, an outline otherwise.
struct TomatoMark: View {
    var filled = true
    var size: CGFloat = 13

    var body: some View {
        Canvas { context, canvas in
            let k = canvas.width / 24
            func point(_ x: CGFloat, _ y: CGFloat) -> CGPoint { CGPoint(x: x * k, y: y * k) }
            var leaves = Path()
            leaves.move(to: point(12, 7.8)); leaves.addLine(to: point(12, 4.2))
            leaves.move(to: point(12, 7.6)); leaves.addQuadCurve(to: point(7.7, 6.3), control: point(10.4, 5.7))
            leaves.move(to: point(12, 7.6)); leaves.addQuadCurve(to: point(16.3, 6.3), control: point(13.6, 5.7))
            if filled {
                let r = 7.6 * k
                context.fill(Path(ellipseIn: CGRect(x: 12 * k - r, y: 14 * k - r, width: r * 2, height: r * 2)), with: .color(Palette.tomato))
                context.stroke(leaves, with: .color(Color(uiColor: UIColor(hex: 0x34C759))), style: StrokeStyle(lineWidth: 1.8 * k, lineCap: .round))
            } else {
                let r = 7 * k
                context.stroke(Path(ellipseIn: CGRect(x: 12 * k - r, y: 14 * k - r, width: r * 2, height: r * 2)), with: .color(Palette.label3), lineWidth: 1.5 * k)
                context.stroke(leaves, with: .color(Palette.label3), style: StrokeStyle(lineWidth: 1.5 * k, lineCap: .round))
            }
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

/// A project's progress: a thin circle that fills like a pie.
struct ProgressRing: View {
    let color: Color
    let fraction: Double
    var size: CGFloat = 21

    var body: some View {
        ZStack {
            Circle().strokeBorder(color, lineWidth: size * 0.08)
            Circle().trim(from: 0, to: max(0, min(1, fraction)))
                .stroke(color, lineWidth: size * 0.3)
                .rotationEffect(.degrees(-90))
                .frame(width: size * 0.3, height: size * 0.3)
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

/// The small ring of the running timer.
struct MiniRing: View {
    let color: Color
    let fraction: Double
    var size: CGFloat = 20

    var body: some View {
        ZStack {
            Circle().stroke(Palette.fill2, lineWidth: 2.6)
            Circle().trim(from: 0, to: max(0, min(1, fraction)))
                .stroke(color, style: StrokeStyle(lineWidth: 2.6, lineCap: .round))
                .rotationEffect(.degrees(-90))
        }
        .padding(1.3)
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

struct ColorDot: View {
    let color: Color
    var size: CGFloat = 8

    var body: some View {
        Circle().fill(color).frame(width: size, height: size).accessibilityHidden(true)
    }
}

/// A rounded card of rows, as in a grouped inset list.
struct GroupCard<Content: View>: View {
    @ViewBuilder var content: Content

    var body: some View {
        VStack(spacing: 0) { content }
            .background(Palette.cell, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
    }
}

/// One row of a `GroupCard`: an icon, a name, a value and a chevron.
struct GroupRow<Icon: View, Accessory: View>: View {
    let title: String
    var value: String?
    var valueColor: Color = Palette.label2
    var titleColor: Color = Palette.label
    var chevron = true
    var last = false
    var minHeight: CGFloat = 48
    @ViewBuilder var icon: Icon
    @ViewBuilder var accessory: Accessory

    var body: some View {
        HStack(spacing: 13) {
            icon.frame(width: 25)
            HStack(spacing: 8) {
                Text(title).font(.system(size: 17)).foregroundStyle(titleColor).lineLimit(1).layoutPriority(1)
                Spacer(minLength: 4)
                accessory
                if let value {
                    Text(value).font(.system(size: 17)).foregroundStyle(valueColor).lineLimit(1)
                }
                if chevron {
                    Image(systemName: "chevron.right").font(.system(size: 13, weight: .semibold)).foregroundStyle(Palette.label3)
                }
            }
            .padding(.trailing, 12)
            .frame(minHeight: minHeight)
            .overlay(alignment: .bottom) { if !last { Divider() } }
        }
        .padding(.leading, 16)
        .contentShape(Rectangle())
    }
}

extension GroupRow where Accessory == EmptyView {
    init(title: String, value: String? = nil, valueColor: Color = Palette.label2, titleColor: Color = Palette.label, chevron: Bool = true, last: Bool = false, minHeight: CGFloat = 48, @ViewBuilder icon: () -> Icon) {
        self.init(title: title, value: value, valueColor: valueColor, titleColor: titleColor, chevron: chevron, last: last, minHeight: minHeight, icon: icon) { EmptyView() }
    }
}

/// The capsule buttons of the focus page and the sheets.
struct CapsuleButtonStyle: ButtonStyle {
    var primary = false
    var height: CGFloat = 50
    @Environment(\.isEnabled) private var enabled

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: height > 40 ? 17 : 15, weight: .semibold))
            .lineLimit(1)
            .minimumScaleFactor(0.8)
            .frame(maxWidth: .infinity, minHeight: height)
            .padding(.horizontal, 8)
            .foregroundStyle(primary ? Palette.onTint : Palette.label)
            .background(primary ? Palette.tint : Palette.fill, in: Capsule())
            .opacity(configuration.isPressed ? 0.7 : enabled ? 1 : 0.45)
    }
}

/// A button that runs one request and cannot be pressed again while it is in flight.
struct AsyncButton<Label: View>: View {
    var role: ButtonRole?
    let action: () async -> Void
    @ViewBuilder var label: Label
    @State private var busy = false

    var body: some View {
        Button(role: role) {
            guard !busy else { return }
            busy = true
            Task {
                await action()
                busy = false
            }
        } label: { label }
        .disabled(busy)
    }
}

extension AsyncButton where Label == Text {
    init(_ title: String, role: ButtonRole? = nil, action: @escaping () async -> Void) {
        self.init(role: role, action: action) { Text(title) }
    }
}

/// The text link under a list: "显示其余 N 件".
struct MoreLink: View {
    let title: String
    var indent: CGFloat = 44
    let action: () async -> Void

    var body: some View {
        AsyncButton(action: action) {
            Text(title).font(.system(size: 15)).foregroundStyle(Palette.tint)
                .frame(maxWidth: .infinity, minHeight: 44, alignment: .leading)
                .padding(.leading, indent)
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}

/// The bell on the first page of each tab, with the number of things waiting.
struct BellButton: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router

    var body: some View {
        Button { router.push(.activity) } label: {
            Image(systemName: "bell")
                .overlay(alignment: .topTrailing) {
                    if model.counts.pending > 0 {
                        Text(Format.cap(model.counts.pending))
                            .font(.system(size: 11, weight: .semibold))
                            .foregroundStyle(.white)
                            .padding(.horizontal, 5)
                            .frame(minWidth: 17, minHeight: 17)
                            .background(Palette.red, in: Capsule())
                            .offset(x: 9, y: -7)
                    }
                }
        }
        .accessibilityLabel(model.counts.pending > 0 ? L("activity.titleWaiting", model.counts.pending) : L("activity.title"))
        .accessibilityIdentifier("bell")
    }
}

/// A line of a day's event on Today, Upcoming and the project page: time, colour bar, title.
struct EventLine: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let event: Event
    /// The day the line stands under, to say "全天" or the start time.
    var showDay = false

    var body: some View {
        let color = Palette.project(model.project(event.projectId)?.color)
        Button { router.open(event) } label: {
            HStack(spacing: 9) {
                Text(time).font(.system(size: 13)).frame(width: showDay ? 92 : 43, alignment: .leading)
                if event.status == .tentative {
                    DashedBar(color: color).frame(width: 3, height: 16)
                } else {
                    RoundedRectangle(cornerRadius: 2).fill(color).frame(width: 3, height: 16)
                }
                Text(event.title).font(.system(size: 15)).lineLimit(1).strikethrough(event.status == .leaving)
                if event.status == .tentative { PendingTag(text: L("cal.pending")) }
                Spacer(minLength: 0)
            }
            .foregroundStyle(Palette.label2)
            .frame(minHeight: 25)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    private var time: String {
        let clock = event.allDay ? L("cal.allDay") : event.start.map(model.zone.hhmm) ?? ""
        guard showDay else { return clock }
        let day = event.allDay ? event.startDate : event.start.map(model.zone.day)
        return [day.map { Format.dayLabel($0, today: model.today) } ?? "", clock].joined(separator: " ")
    }
}

struct DashedBar: View {
    let color: Color

    var body: some View {
        GeometryReader { geo in
            Path { path in
                path.move(to: CGPoint(x: geo.size.width / 2, y: 0))
                path.addLine(to: CGPoint(x: geo.size.width / 2, y: geo.size.height))
            }
            .stroke(color, style: StrokeStyle(lineWidth: geo.size.width, dash: [4, 3]))
        }
    }
}

/// The dashed box that marks something an agent proposed: "待定 14:00".
struct PendingTag: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.system(size: 12))
            .foregroundStyle(Palette.label2)
            .padding(.horizontal, 6)
            .padding(.vertical, 1)
            .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(Palette.label3, style: StrokeStyle(lineWidth: 1, dash: [3, 2])))
            .fixedSize()
    }
}

/// The transient line over the page for a write that failed.
struct NoticeBanner: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        if let notice = model.notice {
            Text(notice)
                .font(.system(size: 15, weight: .medium))
                .multilineTextAlignment(.center)
                .padding(.horizontal, 16)
                .padding(.vertical, 11)
                .background(.regularMaterial, in: Capsule())
                .shadow(color: .black.opacity(0.14), radius: 14, y: 6)
                .padding(.horizontal, 24)
                .padding(.top, 6)
                .transition(.move(edge: .top).combined(with: .opacity))
                .accessibilityIdentifier("notice")
                .task(id: notice) {
                    try? await Task.sleep(for: .seconds(4))
                    if !Task.isCancelled { withAnimation { model.notice = nil } }
                }
        }
    }
}
