import SwiftUI
import UserNotifications

struct HabitsView: View {
    @Environment(AppModel.self) private var model
    @State private var habits: [Habit] = []
    @State private var records: [HabitOccurrence] = []
    @State private var editor: Habit?
    @State private var creating = false
    @State private var from = Date().addingTimeInterval(-6*24*60*60)
    @State private var error = ""
    @State private var reminderState = ""
    @State private var remindersEnabled=HabitNotifications.enabled
    var body: some View {
        List {
            Section {
                
                Button(L(remindersEnabled ? "habit.disableReminders":"habit.enableReminders")) { Task { reminderState=await HabitNotifications.enable();remindersEnabled=HabitNotifications.enabled; await model.syncHabitNotifications() } }
                Text(reminderState.isEmpty ? L("habit.notificationLimit") : reminderState).font(.footnote).foregroundStyle(.secondary)
            }
            Section(L("habit.heading")) {
                ForEach(habits) { h in
                    HStack {
                        Button { editor=h } label: { VStack(alignment:.leading) { Text(h.title);Text("\(h.time) · \(L(h.paused ? "habit.paused":"habit.active"))").font(.caption).foregroundStyle(.secondary) } }
                        Spacer()
                        Button(L(h.paused ? "habit.resume":"habit.pause")) { var body=HabitWrite(h);body.paused.toggle();let saved=body;Task { _=await model.write { try await $0.saveHabit(h.id,saved) };await load() } }.buttonStyle(.borderless)
                    }
                }
            }
            Section(L("habit.history")) {
                DatePicker(L("habit.weekFrom"),selection:$from,displayedComponents:.date)
                Text("\(records.filter{$0.status == "done"}.count)/\(records.filter{$0.status != "future"}.count) · \(L("habit.completed"))").font(.footnote)
                HabitOccurrenceRows(records:records)
            }
            if !error.isEmpty { Text(error).foregroundStyle(.red) }
        }
        .navigationTitle(L("habit.heading"))
        .environment(\.timeZone,model.zone.timeZone)
        .task(id:model.dataVersion) {reminderState=await HabitNotifications.reconcilePermission();remindersEnabled=HabitNotifications.enabled;await load()}
        .task(id:from) { await load() }
        .sheet(isPresented:$creating) { HabitEditor(habit:nil) }
        .sheet(item:$editor) { HabitEditor(habit:$0) }
    }
    private func load() async {
        do { habits=try await model.api.habits();let day=Day(from,in:model.zone.timeZone);records=try await model.api.habitOccurrences(from:day.description,to:day.adding(days:6).description,plan:true);error="" } catch { self.error=L("habit.loadFailed") }
    }
}
struct HabitOccurrenceRows: View {
    @Environment(AppModel.self) private var model
    let records:[HabitOccurrence]
    @State private var selected:HabitOccurrence?
    var body: some View {
        ForEach(records) { o in
            HStack {
                Button { Task { _=await model.write { try await $0.recordHabit(o,.init(status:o.status == "done" ? "open":"done")) } } } label: { Image(systemName:o.status == "done" ? "checkmark.square.fill":"square") }.buttonStyle(.borderless).disabled(o.status == "future")
                Button { selected=o } label: { VStack(alignment:.leading) { Text(o.title);Text("\(o.date) · \(L("habit."+o.status))").font(.caption).foregroundStyle(.secondary) } }.buttonStyle(.plain)
                Spacer()
                if let at=o.scheduledAt { Text(at,format:.dateTime.month().day().hour().minute()).font(.caption) }
            }.padding(.vertical,5)
        }.sheet(item:$selected) { HabitOccurrenceEditor(occurrence:$0) }
    }
}
struct HabitEditor: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let habit:Habit?
    @State private var bodyValue:HabitWrite
    @State private var error=""
    @State private var saving=false
    init(habit:Habit?) { self.habit=habit;_bodyValue=State(initialValue:HabitWrite(habit)) }
    var body: some View {
        NavigationStack {
            Form {
                TextField(L("habit.title"),text:$bodyValue.title)
                Picker(L("field.project"),selection:$bodyValue.projectId) { Text(L("project.none")).tag(String?.none);ForEach(model.projects){Text($0.name).tag(Optional($0.id))} }
                Section(L("habit.repeat")) { ForEach(1...7,id:\.self){d in Toggle(L("habit.day\(d)"),isOn:Binding(get:{bodyValue.weekdays.contains(d)},set:{on in if on {bodyValue.weekdays.append(d)} else {bodyValue.weekdays.removeAll{$0==d}}}))} }
                TextField(L("habit.time"),text:$bodyValue.time).keyboardType(.numbersAndPunctuation)
                Toggle(L("habit.nextDay"),isOn:Binding(get:{bodyValue.dayOffset==1},set:{bodyValue.dayOffset=$0 ? 1:0}))
                Stepper("\(L("habit.duration")) \(bodyValue.durationMinutes)",value:$bodyValue.durationMinutes,in:0...720,step:5)
                Picker(L("habit.reminder"),selection:$bodyValue.reminderMinutes){Text(L("habit.noReminder")).tag(Int?.none);ForEach([0,5,15,30],id:\.self){Text("\($0) \(L("habit.minutesBefore"))").tag(Optional($0))}}
                TextField(L("habit.notes"),text:$bodyValue.notes,axis:.vertical)
                Text(L(habit == nil ? "habit.startsToday":"habit.tomorrow")).font(.footnote)
                if !error.isEmpty {Text(error).foregroundStyle(.red)}
            }.navigationTitle(L(habit == nil ? "habit.new":"habit.rules"))
                .toolbar { ToolbarItem(placement:.cancellationAction){Button(L("common.cancel")){dismiss()}};ToolbarItem(placement:.confirmationAction){Button(L("common.save")){if saving{return};saving=true;var value=bodyValue;if value.time.isEmpty{value.durationMinutes=0;value.dayOffset=0;value.reminderMinutes=nil};let payload=value;let id=habit?.id;Task {let result=await model.write(inline:true){try await $0.saveHabit(id,payload)};saving=false;switch result{case .success:dismiss();case .failure: error=L("habit.saveFailed")}}}.disabled(saving || bodyValue.title.isEmpty)} }
        }
    }
}
struct HabitOccurrenceEditor:View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let occurrence:HabitOccurrence
    @State private var at:Date
    @State private var actual:Date
    init(occurrence:HabitOccurrence){self.occurrence=occurrence;_at=State(initialValue:occurrence.scheduledAt ?? Date());_actual=State(initialValue:occurrence.completedAt ?? Date())}
    var body:some View {NavigationStack{Form{
        Text(occurrence.title)
        DatePicker(L("habit.move"),selection:$at)
        Button(L("habit.move")){save(.init(scheduledAt:at))}
        DatePicker(L("habit.actual"),selection:$actual,in:...Date())
        Button(L("habit.completed")){save(.init(status:"done",completedAt:actual))}.disabled(occurrence.status == "future")
        Button(L("habit.skip")){save(.init(status:"skipped"))}.disabled(occurrence.status == "future")
        Button(L("habit.undo")){save(.init(status:"open"))}
    }.navigationTitle(L("habit.history")).toolbar{ToolbarItem(placement:.cancellationAction){Button(L("common.cancel")){dismiss()}}}}}
    private func save(_ value:HabitRecordWrite){Task{if case .success=await model.write({try await $0.recordHabit(occurrence,value)}){dismiss()}}}
}

struct HabitOccurrenceSheet:View {
 @Environment(AppModel.self) private var model
 let id:String
 let date:String
 @State private var value:HabitOccurrence?
 @State private var failed=false
 var body:some View {Group{if let value {HabitOccurrenceEditor(occurrence:value)}else if failed{Text(L("habit.loadFailed"))}else{ProgressView()}}.task{do{value=try await model.api.habitOccurrences(from:date,to:date).first{$0.habitId==id};failed=value==nil}catch{failed=true}}}
}
