import Foundation

/// What switching an event between timed and all-day does to its dates, in the account's zone.
enum EventDates {
    /// The days a timed stretch touches: what the event covers once it is all-day. `end_date` is inclusive.
    static func allDay(start: Date, end: Date, zone: Zone) -> ClosedRange<Day> {
        zone.days(from: start, to: max(start, end))
    }

    /// The timed stretch for an all-day span: from the first day to the last, at the times of day the
    /// event had before. When that would not end after it starts, it lasts an hour.
    static func timed(days: ClosedRange<Day>, previousStart: Date, previousEnd: Date, zone: Zone) -> (start: Date, end: Date) {
        let start = zone.date(on: days.lowerBound, minutes: zone.minutes(of: previousStart))
        let end = zone.date(on: days.upperBound, minutes: zone.minutes(of: previousEnd))
        guard end > start else {
            return (start, zone.calendar.date(byAdding: .hour, value: 1, to: start) ?? start.addingTimeInterval(3600))
        }
        return (start, end)
    }
}
