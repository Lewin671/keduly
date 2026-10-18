import Foundation
import Observation

enum AppTab: Hashable {
    case items, calendar, focus
}

/// A pushed page. The first page of each tab is not a route.
enum Route: Hashable {
    case matrix, today, upcoming, all, inbox, done, habits
    case project(String)
    case stats
    case activity
    case settings
    case approve
}

/// What a new item starts with, depending on where the add button was pressed.
struct NewItemContext: Hashable {
    var projectId: String?
    var today = false
}

/// Something that rises from the bottom.
enum SheetRoute: Identifiable, Hashable {
    case item(String)
    case newItem(NewItemContext)
    case newEvent(Day)
    case event(Event)
    case suggestion(String)
    case entries([Event], Day)
    case newProject
    case editProject(Project)
    case filing(FocusSession)
    case focusPicker
    case habitDefinition
    case habitOccurrence(String,String)

    var id: String {
        switch self {
        case .item(let id): return "item-\(id)"
        case .newItem: return "new-item"
        case .newEvent: return "new-event"
        case .event(let event): return "event-\(event.entryKey)"
        case .suggestion(let id): return "suggestion-\(id)"
        case .entries: return "entries"
        case .newProject: return "new-project"
        case .editProject(let project): return "project-\(project.id)"
        case .filing(let session): return "filing-\(session.id)"
        case .focusPicker: return "focus-picker"
        case .habitDefinition: return "habit-definition"
        case .habitOccurrence(let id,let date): return "habit-\(id)-\(date)"
        }
    }
}

enum CalendarMode: Hashable {
    case day, month
}

/// What the focus tab is set to start a tomato on.
enum FocusPick: Equatable {
    /// Nothing chosen yet: the first of today's items, or free focus when there is none.
    case automatic
    case item(Item)
    case free
}

/// Where the user is: the tab, each tab's stack, the open sheet.
@MainActor
@Observable
final class Router {
    var tab: AppTab = .items
    /// The items tab opens with Matrix already pushed on the list of lists.
    var itemsPath: [Route] = [.matrix]
    var calendarPath: [Route] = []
    var focusPath: [Route] = []
    var sheet: SheetRoute?
    /// The timer page covers everything, tab bar included.
    var timerExpanded = false
    var calendarMode: CalendarMode = .month
    /// The day the calendar shows; nil follows today.
    var calendarDay: Day?
    var focusPick: FocusPick = .automatic

    /// Tapping the current tab returns to its first page.
    func select(_ tab: AppTab) {
        if tab == self.tab {
            switch tab {
            case .items: itemsPath = []
            case .calendar: calendarPath = []
            case .focus: focusPath = []
            }
        }
        self.tab = tab
    }

    func push(_ route: Route) {
        switch tab {
        case .items: itemsPath.append(route)
        case .calendar: calendarPath.append(route)
        case .focus: focusPath.append(route)
        }
    }

    var top: Route? {
        switch tab {
        case .items: return itemsPath.last
        case .calendar: return calendarPath.last
        case .focus: return focusPath.last
        }
    }

    func reset() {
        tab = .items
        itemsPath = [.matrix]
        calendarPath = []
        focusPath = []
        sheet = nil
        timerExpanded = false
        calendarMode = .month
        calendarDay = nil
        focusPick = .automatic
    }
}

extension Router {
    /// What tapping an entry of the calendar opens: the suggestion behind a tentative or leaving
    /// entry, the item behind a time block, otherwise the event itself.
    func open(_ event: Event) {
        if let id=event.habitId,let date=event.habitDate { sheet = .habitOccurrence(id,date);return }
        if event.status != .confirmed, let suggestion = event.suggestionId {
            sheet = .suggestion(suggestion)
        } else if let item = event.itemId {
            sheet = .item(item)
        } else {
            sheet = .event(event)
        }
    }
}
