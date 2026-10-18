import Foundation

struct Habit: Codable, Identifiable, Hashable, Sendable {
    var id: String
    var title: String
    var notes: String
    var projectId: String?
    var weekdays: [Int]
    var time: String
    var durationMinutes: Int
    var dayOffset: Int
    var reminderMinutes: Int?
    var paused: Bool
    var timezone: String
    var effectiveDate: String
    var version: Int
}
struct HabitOccurrence: Codable, Identifiable, Hashable, Sendable {
    var id: String { habitId + date }
    let planDate: String
    let habitId: String
    let date: String
    let title: String
    let notes: String
    let projectId: String?
    let timezone: String
    let time: String
    let dayOffset: Int
    let durationMinutes: Int
    let scheduledAt: Date?
    let endAt: Date?
    let remindAt: Date?
    let status: String
    let completedAt: Date?
    let skipReason: String
}
struct HabitWrite: Encodable, Sendable {
    var title: String
    var notes: String
    var projectId: String?
    var weekdays: [Int]
    var time: String
    var durationMinutes: Int
    var dayOffset: Int
    var reminderMinutes: Int?
    var paused: Bool
    var version: Int?
    enum CodingKeys: String,CodingKey { case title,notes,projectId,weekdays,time,durationMinutes,dayOffset,reminderMinutes,paused,version }
    func encode(to encoder: any Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(title,forKey:.title);try c.encode(notes,forKey:.notes);try c.encode(projectId,forKey:.projectId)
        try c.encode(weekdays,forKey:.weekdays);try c.encode(time,forKey:.time);try c.encode(durationMinutes,forKey:.durationMinutes)
        try c.encode(dayOffset,forKey:.dayOffset);try c.encode(reminderMinutes,forKey:.reminderMinutes);try c.encode(paused,forKey:.paused);try c.encodeIfPresent(version,forKey:.version)
    }
    init(_ h: Habit? = nil) {
        title=h?.title ?? "";notes=h?.notes ?? "";projectId=h?.projectId;weekdays=h?.weekdays ?? [1,2,3,4,5,6,7]
        time=h?.time ?? "";durationMinutes=h?.durationMinutes ?? 0;dayOffset=h?.dayOffset ?? 0;reminderMinutes=h?.reminderMinutes;paused=h?.paused ?? false;version=h?.version
    }
}
struct HabitRecordWrite: Encodable,Sendable {
    var status: String?
    var completedAt: Date?
    var scheduledAt: Date?
}
extension APIClient {
    func habits() async throws -> [Habit] { try await get("/habits",key:"habits") }
    func habitOccurrences(from: String,to: String,plan:Bool=false) async throws -> [HabitOccurrence] { try await get("/habit-occurrences",["from":from,"to":to,"basis":plan ? "plan":"logical"],key:"occurrences") }
    func saveHabit(_ id: String?,_ body: HabitWrite) async throws -> Habit { try await send(id == nil ? "POST":"PATCH",id.map{"/habits/\($0)"} ?? "/habits",body:body,key:"habit") }
    func recordHabit(_ o: HabitOccurrence,_ body: HabitRecordWrite) async throws -> HabitOccurrence { try await send("PATCH","/habits/\(o.habitId)/occurrences/\(o.date)",body:body,key:"occurrence") }
}
