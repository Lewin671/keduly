import SwiftUI

/// A list of items fetched a batch at a time. Reloading fetches as many as are already shown.
@MainActor
@Observable
final class PagedItems {
    private(set) var items: [Item]?
    private(set) var total = 0
    private(set) var cursor: String?
    private(set) var failed = false

    var remaining: Int { max(0, total - (items?.count ?? 0)) }

    func reload(_ model: AppModel, _ query: APIClient.ItemQuery, first: Int) async {
        var query = query
        query.limit = min(200, max(first, items?.count ?? 0))
        query.cursor = nil
        do {
            let page = try await model.api.items(query)
            items = page.items
            total = page.total
            cursor = page.nextCursor
            failed = false
        } catch is CancellationError {
        } catch APIError.unauthenticated {
        } catch {
            if !Task.isCancelled { failed = true }
        }
    }

    func more(_ model: AppModel, _ query: APIClient.ItemQuery, step: Int) async {
        guard let cursor, let items else { return }
        var query = query
        query.limit = step
        query.cursor = cursor
        do {
            let page = try await model.api.items(query)
            self.items = items + page.items
            total = page.total
            self.cursor = page.nextCursor
            failed = false
        } catch is CancellationError {
        } catch APIError.unauthenticated {
        } catch {
            failed = true
        }
    }
}
