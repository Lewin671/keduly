import Foundation
import UserNotifications

@MainActor
enum HabitNotifications {
    private static var generation = 0
    private static let prefix = "habit."
    static var enabled: Bool { UserDefaults.standard.bool(forKey:"habitReminders") }
    static func enable() async -> String {
        UNUserNotificationCenter.current().delegate=HabitNotificationDelegate.shared
        if enabled { UserDefaults.standard.set(false,forKey:"habitReminders");clear();return L("habit.noReminder") }
        do { let allowed=try await UNUserNotificationCenter.current().requestAuthorization(options:[.alert,.sound]);UserDefaults.standard.set(allowed,forKey:"habitReminders");return L(allowed ? "habit.notificationLimit":"habit.permissionDenied") } catch { return L("habit.permissionDenied") }
    }
    static func select(_ rows:[HabitOccurrence],now:Date)->[HabitOccurrence] {Array(rows.filter{($0.remindAt ?? .distantPast)>now.addingTimeInterval(-60) && ($0.remindAt ?? .distantFuture)<=now.addingTimeInterval(14*24*60*60)}.sorted{$0.remindAt! < $1.remindAt!}.prefix(60))}
    static func signOut(){UserDefaults.standard.set(false,forKey:"habitReminders");clear()}
    static func clear() {
        generation += 1
        UNUserNotificationCenter.current().removeAllPendingNotificationRequests()
        UNUserNotificationCenter.current().removeAllDeliveredNotifications()
    }
    static func reconcilePermission() async -> String {
        let state=await UNUserNotificationCenter.current().notificationSettings()
        return applyPermission(state.authorizationStatus)
    }
    static func applyPermission(_ state:UNAuthorizationStatus)->String {
        if state != .authorized && state != .provisional {signOut();return L(state == .notDetermined ? "habit.permissionInvite":"habit.permissionDenied")}
        return L("habit.notificationLimit")
    }
    static func sync(_ api:APIClient,day:Day) async {
        UNUserNotificationCenter.current().delegate=HabitNotificationDelegate.shared
        _=await reconcilePermission()
        guard enabled else { return }
        generation += 1;let token=generation
        do {
            let rows=try await api.habitOccurrences(from:day.adding(days:-9).description,to:day.adding(days:17).description)
            guard token==generation else {return}
            let center=UNUserNotificationCenter.current()
            let settings=await center.notificationSettings()
            guard token==generation else{return}
            guard settings.authorizationStatus == .authorized || settings.authorizationStatus == .provisional else {signOut();return}
            let pending=await center.pendingNotificationRequests()
            guard token==generation else{return}
            let selected=select(rows,now:Date())
            let keep=Set(selected.map{prefix+$0.id})
            center.removePendingNotificationRequests(withIdentifiers:pending.map(\.identifier).filter{$0.hasPrefix(prefix) && !keep.contains($0)})
            let upcoming=selected.filter{($0.remindAt ?? .distantPast)>Date()}
            for o in upcoming {
                guard token==generation else{return}
                let content=UNMutableNotificationContent();content.title=L("habit.heading");content.body=o.title;content.sound = .default
                content.userInfo=["habit_id":o.habitId,"date":o.date]
                let delay=max(1,o.remindAt!.timeIntervalSinceNow)
                let request=UNNotificationRequest(identifier:prefix+o.id,content:content,trigger:UNTimeIntervalNotificationTrigger(timeInterval:delay,repeats:false))
                try await center.add(request)
                if token != generation {center.removePendingNotificationRequests(withIdentifiers:[request.identifier]);return}
            }
        } catch { /* Keep the last scheduled reminders when offline; the UI discloses this. */ }
    }
}
extension AppModel {
    func syncHabitNotifications() async {guard phase == .signedIn else{return};await HabitNotifications.sync(api,day:today)}
}

final class HabitNotificationDelegate:NSObject,UNUserNotificationCenterDelegate,@unchecked Sendable {
 static let shared=HabitNotificationDelegate()
 func userNotificationCenter(_ center:UNUserNotificationCenter,willPresent notification:UNNotification) async -> UNNotificationPresentationOptions { [.banner,.sound,.list] }
}
