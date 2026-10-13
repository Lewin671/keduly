import SwiftUI

/// What is waiting for an answer, then every recent change, each of which can be undone.
struct ActivityView: View {
    @Environment(AppModel.self) private var model

    private struct Content: Sendable {
        var suggestions: [Suggestion]
        var activities: [Activity]
        var cursor: String?
    }

    @State private var loader = Loader<Content>()

    var body: some View {
        Screen(failed: loader.failed, retry: load) {
            LargeTitle(L("activity.title"))
            if let content = loader.value {
                if content.suggestions.isEmpty && content.activities.isEmpty {
                    Blank(title: L("activity.empty"), text: L("activity.emptyTip"))
                }
                if !content.suggestions.isEmpty {
                    SectionHeader(L("activity.todo"), trailing: String(content.suggestions.count))
                    if content.suggestions.count > 1 {
                        AsyncButton(L("activity.acceptAll")) {
                            _ = await model.write { try await $0.acceptAllSuggestions() }
                        }
                        .font(.app(15))
                        .frame(minHeight: 44)
                        .accessibilityIdentifier("accept-all")
                    }
                    ForEach(content.suggestions) { suggestion in
                        VStack(alignment: .leading, spacing: 6) {
                            Text(suggestion.title).font(.app(17)).padding(.top, 10)
                            SuggestionCard(title: SuggestionText.what(suggestion, zone: model.zone, today: model.today), reason: suggestion.reason,
                                           actor: "\(suggestion.actor.name) · \(Format.stamp(suggestion.createdAt, zone: model.zone, today: model.today))",
                                           id: suggestion.id)
                        }
                        .padding(.bottom, 4)
                    }
                }
                if !content.activities.isEmpty {
                    SectionHeader(L("activity.recent"))
                    ForEach(Array(content.activities.enumerated()), id: \.element.id) { index, activity in
                        row(activity, last: index == content.activities.count - 1)
                    }
                    if let cursor = content.cursor {
                        MoreLink(title: L("activity.older"), indent: 0) { await more(cursor) }
                    }
                }
            } else if !loader.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("activity.title"))
        .task(id: model.dataVersion) { await load() }
    }

    private func row(_ activity: Activity, last: Bool) -> some View {
        HStack(alignment: .top, spacing: 10) {
            VStack(alignment: .leading, spacing: 2) {
                Text(activity.summary).font(.app(17)).foregroundStyle(activity.undone ? Palette.label2 : Palette.label)
                    .strikethrough(activity.undone)
                if let reason = activity.reason, !reason.isEmpty {
                    Text(reason).font(.app(13)).foregroundStyle(Palette.label2)
                }
                Text("\(activity.actor.kind == .user ? L("activity.you") : activity.actor.name) · \(Format.stamp(activity.createdAt, zone: model.zone, today: model.today))")
                    .font(.app(13)).foregroundStyle(Palette.label2)
            }
            Spacer(minLength: 0)
            if activity.undone {
                VStack(alignment: .trailing, spacing: 0) {
                    Text(L("activity.undone")).font(.app(13)).foregroundStyle(Palette.label3).padding(.top, 2)
                    AsyncButton(L("activity.redo")) {
                        let id = activity.id
                        _ = await model.write { try await $0.redoActivity(id) }
                    }
                    .font(.app(15))
                    .frame(minWidth: 44, minHeight: 36, alignment: .trailing)
                    .accessibilityIdentifier("redo-\(activity.summary)")
                }
            } else if activity.undoable {
                AsyncButton(L("activity.undo")) {
                    let id = activity.id
                    _ = await model.write { try await $0.undoActivity(id) }
                }
                .font(.app(15))
                .frame(minWidth: 44, minHeight: 44, alignment: .topTrailing)
                .accessibilityIdentifier("undo-\(activity.summary)")
            }
        }
        .padding(.vertical, 10)
        .overlay(alignment: .bottom) { if !last { Divider() } }
    }

    private func load() async {
        // Reloading keeps as many entries as were on screen, a page at a time.
        let wanted = loader.value?.activities.count ?? 0
        await loader.load(model) { api in
            async let suggestions = api.pendingSuggestions()
            var page = try await api.activity(cursor: nil)
            var activities = page.activities
            while activities.count < wanted, let cursor = page.nextCursor {
                page = try await api.activity(cursor: cursor)
                activities += page.activities
            }
            return Content(suggestions: try await suggestions, activities: activities, cursor: page.nextCursor)
        }
    }

    private func more(_ cursor: String) async {
        guard let page = try? await model.api.activity(cursor: cursor), var content = loader.value else { return }
        content.activities += page.activities
        content.cursor = page.nextCursor
        loader.value = content
    }
}
