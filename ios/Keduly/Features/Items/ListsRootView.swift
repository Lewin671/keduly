import SwiftUI

/// The first page of the items tab: the lists, then the projects by area.
struct ListsRootView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @State private var query = ""
    @State private var results = PagedItems()

    private var searching: Bool { !query.trimmingCharacters(in: .whitespaces).isEmpty }

    var body: some View {
        Screen(failed: searching && results.failed, grouped: true, retry: { await search() }) {
            LargeTitle(L("mode.items"))
            searchField
            if searching {
                searchResults
            } else {
                smartLists
                projectGroups
                GroupCard {
                    Button { router.sheet = .newProject } label: {
                        GroupRow(title: L("project.new"), titleColor: Palette.tint, chevron: false, last: true) {
                            Image(systemName: "plus").font(.system(size: 19, weight: .medium)).foregroundStyle(Palette.tint)
                        }
                    }
                    .buttonStyle(.plain)
                    .accessibilityIdentifier("new-project")
                }
                .padding(.top, 22)
            }
        }
        .navigationTitle(L("mode.items"))
        .toolbar {
            ToolbarItemGroup(placement: .topBarTrailing) {
                BellButton()
                Button { router.push(.settings) } label: { Image(systemName: "gearshape") }
                    .accessibilityLabel(L("settings.title"))
                    .accessibilityIdentifier("settings")
            }
        }
        .task(id: SearchKey(query: query, version: model.dataVersion)) {
            guard searching else { return }
            try? await Task.sleep(for: .milliseconds(300))
            if !Task.isCancelled { await search() }
        }
    }

    private struct SearchKey: Equatable {
        let query: String
        let version: Int
    }

    private func search() async {
        guard searching else { return }
        await results.reload(model, .init(status: "any", search: query.trimmingCharacters(in: .whitespaces)), first: 50)
    }

    private var searchField: some View {
        HStack(spacing: 6) {
            Image(systemName: "magnifyingglass").font(.system(size: 16, weight: .medium)).foregroundStyle(Palette.label2)
            TextField(L("search.placeholder"), text: $query)
                .font(.system(size: 17))
                .submitLabel(.search)
                .autocorrectionDisabled()
                .accessibilityIdentifier("search")
            if !query.isEmpty {
                Button { query = "" } label: { Image(systemName: "xmark.circle.fill").foregroundStyle(Palette.label3) }
                    .buttonStyle(.plain)
                    .accessibilityLabel(L("common.clear"))
            }
        }
        .padding(.horizontal, 9)
        .frame(height: 36)
        .background(Palette.fill, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
        .padding(.top, 6)
        .padding(.bottom, 18)
    }

    @ViewBuilder private var searchResults: some View {
        if let items = results.items {
            if items.isEmpty {
                Blank(title: L("search.empty"), text: L("search.emptyTip"), top: 80)
            } else {
                GroupCard {
                    ItemRows(items: items).padding(.horizontal, 16)
                }
                if results.remaining > 0 {
                    MoreLink(title: L("list.rest", results.remaining), indent: 16) {
                        await results.more(model, .init(status: "any", search: query.trimmingCharacters(in: .whitespaces)), step: 50)
                    }
                }
            }
        } else if !results.failed {
            ProgressView().frame(maxWidth: .infinity).padding(.top, 40)
        }
    }

    private var smartLists: some View {
        GroupCard {
            smart(.matrix, "square.grid.2x2", L("nav.matrix"), Palette.orange, nil)
            smart(.today, "star.fill", L("nav.today"), Palette.star, model.counts.today)
            smart(.upcoming, "calendar", L("nav.upcoming"), Palette.red, nil)
            smart(.all, "square.stack.3d.up", L("nav.all"), Palette.teal, nil)
            smart(.inbox, "tray", L("nav.inbox"), Palette.blue, model.counts.inbox)
            smart(.done, "checkmark.square", L("nav.done"), Palette.green, nil, last: true)
        }
    }

    private func smart(_ route: Route, _ symbol: String, _ title: String, _ color: Color, _ count: Int?, last: Bool = false) -> some View {
        Button { router.push(route) } label: {
            GroupRow(title: title, value: count.flatMap { $0 > 0 ? Format.cap($0) : nil }, last: last) {
                Image(systemName: symbol).font(.system(size: 19, weight: .medium)).foregroundStyle(color)
            }
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("list-\(title)")
    }

    @ViewBuilder private var projectGroups: some View {
        let loose = model.projects.filter { project in !model.areas.contains { $0.id == project.areaId } }
        if !loose.isEmpty {
            projectCard(loose).padding(.top, 22)
        }
        ForEach(model.areas) { area in
            let list = model.projects.filter { $0.areaId == area.id }
            if !list.isEmpty {
                Text(area.name).font(.system(size: 13)).foregroundStyle(Palette.label2)
                    .padding(.top, 22).padding(.bottom, 7).padding(.horizontal, 16)
                    .accessibilityAddTraits(.isHeader)
                projectCard(list)
            }
        }
    }

    private func projectCard(_ projects: [Project]) -> some View {
        GroupCard {
            ForEach(Array(projects.enumerated()), id: \.element.id) { index, project in
                Button { router.push(.project(project.id)) } label: {
                    GroupRow(title: project.name, value: project.openCount > 0 ? Format.cap(project.openCount) : nil, last: index == projects.count - 1) {
                        let total = project.openCount + project.doneCount
                        ProgressRing(color: Palette.project(project.color), fraction: total > 0 ? Double(project.doneCount) / Double(total) : 0)
                    }
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("project-\(project.name)")
            }
        }
    }
}
