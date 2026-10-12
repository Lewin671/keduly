import SwiftUI

@main
struct KedulyApp: App {
    @State private var model = AppModel(api: APIClient(store: KeychainSessionStore()))
    @State private var router = Router()
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(model)
                .environment(router)
                .tint(Palette.tint)
                .task {
                    #if DEBUG
                    if await DebugLaunch.run(model, router) { return }
                    #endif
                    await model.start()
                }
                .onChange(of: scenePhase) { _, phase in
                    model.setActive(phase == .active)
                }
        }
    }
}

/// The sign-in screen, the app, or the wait in between.
struct RootView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router

    var body: some View {
        Group {
            switch model.phase {
            case .launching(let failed):
                if failed {
                    VStack(spacing: 14) {
                        Text(L("error.network")).font(.system(size: 17)).foregroundStyle(Palette.label2)
                        AsyncButton(L("common.retry")) { await model.enter() }
                            .buttonStyle(CapsuleButtonStyle(primary: true))
                            .frame(width: 180)
                        AsyncButton(L("account.signOut")) { await model.signOut() }
                            .font(.system(size: 15))
                            .frame(minHeight: 44)
                    }
                } else {
                    ProgressView()
                }
            case .signedOut:
                SignInView()
            case .signedIn:
                MainView()
                    // Every date and time is shown and picked in the account's zone, not the device's.
                    .environment(\.timeZone, model.zone.timeZone)
                    .environment(\.calendar, model.zone.calendar)
            }
        }
        .environment(\.locale, Locale(identifier: "zh-Hans"))
        .onChange(of: model.phase) { _, phase in
            if phase == .signedOut { router.reset() }
        }
    }
}
