import SwiftUI

/// A new item. Only the title is required; the chips are optional.
struct NewItemSheet: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let context: NewItemContext

    @State private var title = ""
    @State private var today = false
    @State private var due: Day?
    @State private var pickingDue = false
    @State private var estimate: Int?
    @State private var projectId: String?
    @State private var important = false
    @State private var error: String?
    @State private var busy = false
    @FocusState private var focused: Bool

    private var trimmed: String { title.trimmingCharacters(in: .whitespacesAndNewlines) }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Button(L("common.cancel")) { dismiss() }
                Spacer()
                Text(L("item.new")).font(.app(17, .semibold))
                Spacer()
                Button(L("common.add")) { Task { await add() } }
                    .fontWeight(.semibold)
                    .disabled(busy || trimmed.isEmpty)
                    .accessibilityIdentifier("new-item-add")
            }
            .frame(height: 48)
            .padding(.horizontal, 4)

            TextField(L("item.placeholder"), text: $title, axis: .vertical)
                .font(.app(22, .semibold))
                .lineLimit(1...3)
                .focused($focused)
                .submitLabel(.done)
                .onChange(of: title) { _, new in
                    // Return adds the item; a title has no line breaks.
                    if new.contains("\n") {
                        title = new.replacingOccurrences(of: "\n", with: "")
                        Task { await add() }
                    }
                }
                .padding(.horizontal, 4)
                .padding(.top, 8)
                .accessibilityIdentifier("new-item-title")

            if let error {
                Text(error).font(.app(13)).foregroundStyle(Palette.red).padding(.horizontal, 4).padding(.top, 6)
            }

            FlowLayout(spacing: 8) {
                chip("star.fill", L("day.today"), set: today) { today.toggle() }
                    .accessibilityIdentifier("chip-today")
                Menu {
                    Button(L("day.today")) { due = model.today }
                    Button(L("day.tomorrow")) { due = model.today.adding(days: 1) }
                    Button(L("due.inAWeek")) { due = model.today.adding(days: 7) }
                    Button(L("due.pick")) { due = due ?? model.today; pickingDue = true }
                    if due != nil { Button(L("common.clear"), role: .destructive) { due = nil; pickingDue = false } }
                } label: {
                    chipLabel("flag", due.map { L("due.label", Format.dayLabel($0, today: model.today)) } ?? L("field.due"), set: due != nil)
                }
                .accessibilityIdentifier("chip-due")
                Menu {
                    ForEach(Estimates.presets, id: \.self) { minutes in
                        Button(Format.dur(minutes)) { estimate = minutes }
                    }
                    if estimate != nil { Button(L("common.clear"), role: .destructive) { estimate = nil } }
                } label: {
                    chipLabel("clock", estimate.map(Format.dur) ?? L("field.estimate"), set: estimate != nil)
                }
                .accessibilityIdentifier("chip-estimate")
                Menu {
                    Button(L("nav.inbox")) { projectId = nil }
                    ForEach(model.projects) { project in
                        Button(project.name) { projectId = project.id }
                    }
                } label: {
                    chipLabel("tray", model.project(projectId)?.name ?? L("nav.inbox"), set: projectId != nil)
                }
                .accessibilityIdentifier("chip-project")
                chip("exclamationmark", L("item.important"), set: important, tint: Palette.orange) { important.toggle() }
                    .accessibilityIdentifier("chip-important")
            }
            .padding(.horizontal, 4)
            .padding(.top, 14)

            if pickingDue, due != nil {
                DatePicker(L("due.date"), selection: dayBinding, displayedComponents: .date)
                    .datePickerStyle(.graphical)
                    .padding(.top, 6)
            }
            Spacer(minLength: 0)
        }
        .padding(.horizontal, 16)
        .padding(.top, 8)
        .background(Palette.grouped)
        .presentationDetents(pickingDue ? [.large] : [.height(230), .large])
        .presentationDragIndicator(.visible)
        .onAppear {
            projectId = context.projectId
            today = context.today
            focused = true
        }
    }

    private var dayBinding: Binding<Date> {
        Binding(
            get: { model.zone.date(on: due ?? model.today, minutes: 12 * 60) },
            set: { due = model.zone.day(of: $0) }
        )
    }

    private func chip(_ symbol: String, _ text: String, set: Bool, tint: Color = Palette.label, action: @escaping () -> Void) -> some View {
        Button(action: action) { chipLabel(symbol, text, set: set, tint: tint) }
            .buttonStyle(.plain)
            .accessibilityAddTraits(set ? .isSelected : [])
    }

    private func chipLabel(_ symbol: String, _ text: String, set: Bool, tint: Color = Palette.label) -> some View {
        HStack(spacing: 6) {
            Image(systemName: symbol).font(.app(13, .medium))
            Text(text).font(.app(15)).lineLimit(1)
        }
        .foregroundStyle(set ? tint : Palette.label2)
        .padding(.horizontal, 12)
        .frame(minHeight: 34)
        .frame(maxWidth: 240)
        .background(Palette.cell, in: Capsule())
        .contentShape(Capsule())
    }

    private func add() async {
        guard !busy, !trimmed.isEmpty else { return }
        busy = true
        defer { busy = false }
        let item = NewItem(title: trimmed, projectId: projectId, estimateMinutes: estimate, plannedDate: today ? model.today : nil,
                           dueDate: due, important: important ? true : nil)
        switch await model.write(inline: true, { try await $0.createItem(item) }) {
        case .success: dismiss()
        case .failure(let failure): error = failure.status == 400 ? failure.message : nil
        }
    }
}

enum Estimates {
    static let presets = [15, 30, 45, 60, 90, 120, 180, 240]
}

/// Lays chips out in rows, wrapping to the next row when one is full.
struct FlowLayout: Layout {
    var spacing: CGFloat = 8

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = arrange(proposal.width ?? .infinity, subviews)
        return CGSize(width: proposal.width ?? rows.map(\.width).max() ?? 0, height: rows.last.map { $0.y + $0.height } ?? 0)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        for row in arrange(bounds.width, subviews) {
            for (index, x) in row.items {
                subviews[index].place(at: CGPoint(x: bounds.minX + x, y: bounds.minY + row.y), proposal: .unspecified)
            }
        }
    }

    private struct Row {
        var items: [(Int, CGFloat)] = []
        var y: CGFloat = 0
        var width: CGFloat = 0
        var height: CGFloat = 0
    }

    private func arrange(_ width: CGFloat, _ subviews: Subviews) -> [Row] {
        var rows: [Row] = [Row()]
        for (index, subview) in subviews.enumerated() {
            let size = subview.sizeThatFits(.unspecified)
            if !rows[rows.count - 1].items.isEmpty, rows[rows.count - 1].width + spacing + size.width > width {
                let last = rows[rows.count - 1]
                rows.append(Row(y: last.y + last.height + spacing))
            }
            var row = rows[rows.count - 1]
            let x = row.items.isEmpty ? 0 : row.width + spacing
            row.items.append((index, x))
            row.width = x + size.width
            row.height = max(row.height, size.height)
            rows[rows.count - 1] = row
        }
        return rows
    }
}
