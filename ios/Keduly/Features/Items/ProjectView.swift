import SwiftUI

/// A project: ring and name, notes, the summary line, the next events, then the items under headings.
struct ProjectView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let id: String

    private struct Content: Sendable {
        let detail: ProjectDetail
        let items: [Item]
    }

    @State private var loader = Loader<Content>()
    @State private var done = PagedItems()
    @State private var showDone = false

    private var doneQuery: APIClient.ItemQuery { .init(status: "done", projectId: id) }

    var body: some View {
        Screen(failed: loader.failed, retry: load) {
            if let content = loader.value {
                page(content)
            } else if loader.failed {
                LargeTitle(model.project(id)?.name ?? "")
            } else {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(model.project(id)?.name ?? "")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Menu {
                    if let project = loader.value?.detail.project {
                        Button(L("project.edit"), systemImage: "pencil") { router.sheet = .editProject(project) }
                    }
                } label: {
                    Image(systemName: "ellipsis")
                }
                .accessibilityLabel(L("project.menu"))
                .accessibilityIdentifier("project-menu")
            }
        }
        .task(id: model.dataVersion) { await load() }
    }

    private func load() async {
        let id = id
        await loader.load(model) { api in
            async let detail = api.project(id)
            async let items = api.items(.init(projectId: id, limit: 200))
            return Content(detail: try await detail, items: try await items.items)
        }
        if showDone { await done.reload(model, doneQuery, first: 10) }
    }

    @ViewBuilder private func page(_ content: Content) -> some View {
        let project = content.detail.project
        let color = Palette.project(project.color)
        let total = project.openCount + project.doneCount
        LargeTitle(text: project.name, lineLimit: 3) {
            ProgressRing(color: color, fraction: total > 0 ? Double(project.doneCount) / Double(total) : 0, size: 26)
        }
        if !project.notes.isEmpty {
            Text(project.notes).font(.system(size: 15)).foregroundStyle(Palette.label2).lineLimit(3).padding(.bottom, 10)
        }
        if content.items.isEmpty {
            Blank(title: L("project.empty"), text: L("project.emptyTip"), top: project.doneCount > 0 ? 60 : 110)
                .padding(.bottom, 30)
        } else {
            Text(summary(content)).font(.system(size: 15)).foregroundStyle(Palette.label2).padding(.bottom, 6)
            if !content.detail.upcomingEvents.isEmpty {
                VStack(spacing: 0) {
                    ForEach(content.detail.upcomingEvents.prefix(2), id: \.entryKey) { EventLine(event: $0, showDay: true) }
                }
                .padding(.top, 4)
            }
            let loose = content.items.filter { item in !content.detail.headings.contains { $0.id == item.headingId } }
            if !loose.isEmpty {
                Divider().padding(.top, 10)
                ItemRows(items: loose, showProject: false)
            }
            ForEach(content.detail.headings.sorted { $0.position < $1.position }) { heading in
                let items = content.items.filter { $0.headingId == heading.id }
                if !items.isEmpty {
                    SectionHeader(heading.name, trailing: String(items.count))
                    ItemRows(items: items, showProject: false)
                }
            }
        }
        if project.doneCount > 0 {
            if showDone, let items = done.items {
                SectionHeader(L("nav.done"), trailing: String(done.total))
                ItemRows(items: items, showProject: false)
                if done.remaining > 0 {
                    MoreLink(title: L("list.more", min(30, done.remaining), done.remaining), indent: 34) {
                        await done.more(model, doneQuery, step: 30)
                    }
                }
            } else {
                MoreLink(title: L("project.showDone", project.doneCount), indent: 34) {
                    await done.reload(model, doneQuery, first: 10)
                    showDone = true
                }
            }
        }
    }

    private func summary(_ content: Content) -> String {
        var text = L("project.open", content.detail.project.openCount)
        text += content.detail.unplannedCount > 0 ? L("project.unplanned", content.detail.unplannedCount) : L("project.allPlanned")
        if content.detail.focusWeekMinutes > 0 { text += L("project.focusWeek", Format.span(content.detail.focusWeekMinutes)) }
        return text
    }
}

/// Creating a project, or changing its name, colour, area and notes.
struct ProjectEditor: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let project: Project?

    @State private var name = ""
    @State private var color: ProjectColor?
    @State private var areaId: String?
    @State private var notes = ""
    @State private var error: String?
    @State private var busy = false

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField(L("project.name"), text: $name).accessibilityIdentifier("project-name")
                    TextField(L("item.notes"), text: $notes, axis: .vertical).lineLimit(1...5)
                } footer: {
                    if let error { Text(error).foregroundStyle(Palette.red) }
                }
                Section(L("project.color")) {
                    HStack {
                        ForEach(ProjectColor.allCases, id: \.self) { option in
                            Button { color = option } label: {
                                Circle().fill(Palette.project(option)).frame(width: 28, height: 28)
                                    .overlay { if color == option { Image(systemName: "checkmark").font(.system(size: 13, weight: .bold)).foregroundStyle(.white) } }
                                    .frame(maxWidth: .infinity, minHeight: 44)
                            }
                            .buttonStyle(.plain)
                            .accessibilityLabel(L("color.\(option.rawValue)"))
                            .accessibilityAddTraits(color == option ? .isSelected : [])
                        }
                    }
                }
                if !model.areas.isEmpty {
                    Section {
                        Picker(L("project.area"), selection: $areaId) {
                            Text(L("area.none")).tag(String?.none)
                            ForEach(model.areas) { Text($0.name).tag(String?.some($0.id)) }
                        }
                    }
                }
            }
            .navigationTitle(project == nil ? L("project.new") : L("project.edit"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button(L("common.cancel")) { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button(project == nil ? L("common.create") : L("common.save")) { Task { await save() } }
                        .disabled(busy || name.trimmingCharacters(in: .whitespaces).isEmpty)
                        .accessibilityIdentifier("project-save")
                }
            }
        }
        .presentationDetents([.medium, .large])
        .onAppear {
            guard let project else { return }
            name = project.name
            color = project.color
            areaId = project.areaId
            notes = project.notes
        }
    }

    private func save() async {
        busy = true
        defer { busy = false }
        let name = name.trimmingCharacters(in: .whitespaces), color = color, areaId = areaId, notes = notes
        let result: Result<Project, APIError>
        if let project {
            let patch = ProjectPatch(name: .set(name), color: color.map(Patch.set) ?? .unchanged, areaId: Patch(areaId), notes: .set(notes))
            result = await model.write(inline: true) { try await $0.updateProject(project.id, patch) }
        } else {
            result = await model.write(inline: true) { try await $0.createProject(name: name, color: color, areaId: areaId, notes: notes) }
        }
        switch result {
        case .success: dismiss()
        case .failure(let failure): error = failure.status == 400 ? failure.message : nil
        }
    }
}
