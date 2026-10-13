import SwiftUI

/// Every open item by importance and urgency: the four quadrants stacked.
struct MatrixView: View {
    @Environment(AppModel.self) private var model
    @State private var loader = Loader<Quadrants>()
    /// Quadrants whose every item was asked for.
    @State private var full: [QuadrantKey: [Item]] = [:]

    var body: some View {
        Screen(failed: loader.failed, retry: load) {
            LargeTitle(L("nav.matrix"))
            if let quadrants = loader.value {
                if QuadrantKey.allCases.allSatisfy({ quadrants[$0].total == 0 }) {
                    Blank(title: L("all.empty"), text: L("matrix.emptyTip"))
                } else {
                    ForEach(QuadrantKey.allCases, id: \.self) { key in
                        section(key, quadrants[key])
                    }
                }
            } else if !loader.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("nav.matrix"))
        .toolbar { ToolbarItem(placement: .topBarTrailing) { BellButton() } }
        .task(id: model.dataVersion) { await load() }
    }

    @ViewBuilder private func section(_ key: QuadrantKey, _ quadrant: Quadrant) -> some View {
        let items = full[key] ?? quadrant.items
        SectionHeader(L("matrix.\(key.rawValue)"), note: L("matrix.\(key.rawValue)Label"),
                      trailing: key == .plan && quadrant.unplanned > 0 ? L("matrix.unplanned", quadrant.unplanned) : String(quadrant.total))
        if items.isEmpty {
            Text(L("matrix.none")).font(.app(15)).foregroundStyle(Palette.label3).padding(.leading, 34).padding(.top, 12).padding(.bottom, 4)
        } else {
            ItemRows(items: items)
            if quadrant.total > items.count {
                MoreLink(title: L("list.rest", quadrant.total - items.count), indent: 34) { await expand(key) }
            }
        }
    }

    private func load() async {
        await loader.load(model) { try await $0.matrix() }
        for key in full.keys { await expand(key) }
    }

    private func expand(_ key: QuadrantKey) async {
        if let page = try? await model.api.items(.init(quadrant: key), upTo: .max) {
            full[key] = page.items
        }
    }
}
