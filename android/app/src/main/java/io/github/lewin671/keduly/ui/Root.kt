// The frame of the app: which of the three states it is in (starting, signed out, signed in), and
// when signed in the Material chrome around the pages: navigation bar, add button, timer strip,
// sheets and the full-screen timer.
package io.github.lewin671.keduly.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.LifecycleStartEffect
import io.github.lewin671.keduly.update.UpdateState
import io.github.lewin671.keduly.update.Updater
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.ViewModelStore
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.LocalViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavGraphBuilder
import androidx.navigation.NavHostController
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.navigation
import androidx.navigation.compose.rememberNavController
import io.github.lewin671.keduly.Launch
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.normalizeServer
import io.github.lewin671.keduly.core.FocusMath
import io.github.lewin671.keduly.core.Times

/** Holds the view models of one signed-in session that do not belong to a page. */
private class SessionStoreOwner : ViewModelStoreOwner {
    override val viewModelStore = ViewModelStore()
}

private class Tab(val graph: String, val root: String, val label: Int, val icon: ImageVector)

private val TABS = listOf(
    Tab("tab_items", "lists", R.string.mode_items, KIcons.Checklist),
    Tab("tab_calendar", "calendar", R.string.mode_calendar, KIcons.Calendar),
    Tab("tab_focus", "focus", R.string.mode_focus, KIcons.Timer),
)

@Composable
fun App(launch: Launch?) {
    val app: AppModel = viewModel()
    val phase by app.phase.collectAsStateWithLifecycle()
    val generation by app.generation.collectAsStateWithLifecycle()
    LifecycleStartEffect(Unit) {
        app.setForeground(true)
        onStopOrDispose { app.setForeground(false) }
    }
    LaunchedEffect(Unit) {
        val server = launch?.server?.let(::normalizeServer)
        if (server != null && launch.email != null && launch.password != null) app.signIn(server, launch.email, launch.password, null) else app.start()
    }
    CompositionLocalProvider(LocalApp provides app) {
        Surface(color = K.bg, contentColor = K.label) {
            when (phase) {
                Phase.Starting -> Starting()
                Phase.SignedOut -> SignIn()
                // A page asked for by a debug launch applies to the session it started, not to later ones.
                Phase.SignedIn -> key(generation) { Shell(if (generation == 0) launch?.route else null) }
            }
        }
    }
}

/** A stored session is being checked. Without a connection it says so and offers to retry. */
@Composable
private fun Starting() {
    val app = LocalApp.current
    val error by app.bootError.collectAsStateWithLifecycle()
    Column(Modifier.fillMaxSize().padding(32.dp), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
        val message = error
        if (message == null) {
            CircularProgressIndicator(Modifier.size(28.dp), color = K.label3, strokeWidth = 2.5.dp)
        } else {
            Text(message, fontSize = 15.sp, color = K.label2)
            Spacer(Modifier.height(16.dp))
            PillButton(stringResource(R.string.retry), Modifier.width(200.dp), primary = true) { app.start() }
            TextButton(onClick = app::signOut) { Text(stringResource(R.string.account_sign_out), color = K.tint) }
        }
    }
}

@Composable
private fun Shell(startRoute: String?) {
    val app = LocalApp.current
    val boot by app.boot.collectAsStateWithLifecycle()
    val today by app.today.collectAsStateWithLifecycle()
    val user = boot?.user ?: return
    val day = today ?: return
    val configuration = LocalConfiguration.current
    val resources = LocalContext.current.resources
    val fmt = remember(user.timezone, day, user.focusMinutes, configuration) { Fmt(resources, app.zone(), day, user.focusMinutes) }

    val session = remember { SessionStoreOwner() }
    DisposableEffect(session) { onDispose { session.viewModelStore.clear() } }

    val controller = rememberNavController()
    val entry by controller.currentBackStackEntryAsState()
    val route = entry?.destination?.route
    val tab = TABS.firstOrNull { t -> entry?.destination?.hierarchy?.any { it.route == t.graph } == true } ?: TABS[0]
    val nav = remember(controller, tab) {
        Nav(
            go = { controller.navigate(it) { launchSingleTop = true } },
            back = { controller.popBackStack() },
            activity = { controller.navigate("${tab.graph}/activity") { launchSingleTop = true } },
            calendarDay = { day ->
                // The day view of that day; null is today.
                app.calendarMode.value = 0
                app.calendarDate.value = day
                controller.switchTab(TABS[1])
            },
        )
    }
    LaunchedEffect(route) { app.sticky.clear() }

    // The app opens with the matrix already pushed on the list of lists.
    var opened by rememberSaveable { mutableStateOf(false) }
    LaunchedEffect(Unit) {
        if (!opened) {
            opened = true
            controller.open(app, startRoute ?: "matrix")
        }
    }

    val focus by app.focus.collectAsStateWithLifecycle()
    val zen by app.zen.collectAsStateWithLifecycle()
    val running = focus?.takeIf { it.state != "idle" }
    val snackbar = remember { SnackbarHostState() }
    LaunchedEffect(Unit) { app.messages.collect { snackbar.showSnackbar(it) } }
    // A newer release found in the background is mentioned once; Settings offers it from then on.
    LaunchedEffect(Unit) {
        var told: String? = null
        Updater.state.collect { state ->
            if (state is UpdateState.Available && state.release.version != told) {
                told = state.release.version
                app.messages.tryEmit(app.text(R.string.update_toast, state.release.version))
            }
        }
    }

    CompositionLocalProvider(LocalFmt provides fmt, LocalNav provides nav) {
        FocusWatcher()
        Box(Modifier.fillMaxSize()) {
            Scaffold(
                containerColor = K.bg,
                contentWindowInsets = WindowInsets(0, 0, 0, 0),
                snackbarHost = { SnackbarHost(snackbar) },
                bottomBar = {
                    Column {
                        // Away from the timer page a running timer shows above the navigation bar.
                        if (running != null && route != "focus") TimerStrip { app.zen.value = true }
                        NavigationBar(containerColor = K.wash(K.label, 0.04f), tonalElevation = 0.dp) {
                            for (t in TABS) {
                                NavigationBarItem(
                                    selected = t === tab,
                                    onClick = { if (t === tab) controller.popBackStack(t.root, false) else controller.switchTab(t) },
                                    icon = { Icon(t.icon, null, Modifier.size(25.dp)) },
                                    label = { Text(stringResource(t.label)) },
                                    colors = NavigationBarItemDefaults.colors(
                                        selectedIconColor = K.tint, selectedTextColor = K.tint, indicatorColor = K.wash(K.tint, 0.16f, K.wash(K.label, 0.04f)),
                                        unselectedIconColor = K.label2, unselectedTextColor = K.label2,
                                    ),
                                )
                            }
                        }
                    }
                },
                floatingActionButton = {
                    // Settings and approving a device are pages of values to edit; the add button would sit on top of them.
                    if (route != "settings" && route != "approve") FloatingActionButton(
                        onClick = {
                            app.sheet.value = when {
                                tab === TABS[1] -> Sheet.EventForm(null, app.calendarDate.value ?: day)
                                route == "project/{id}" -> Sheet.NewItem(projectId = entry?.arguments?.getString("id"))
                                route == "today" -> Sheet.NewItem(today = true)
                                route == "habits" -> Sheet.NewHabit
                                else -> Sheet.NewItem()
                            }
                        },
                        containerColor = K.tint, contentColor = K.onTint,
                    ) { Icon(KIcons.Plus, stringResource(if (tab === TABS[1]) R.string.event_new else R.string.item_new), Modifier.size(27.dp)) }
                },
            ) { padding ->
                NavHost(controller, startDestination = TABS[0].graph, modifier = Modifier.padding(padding).consumeWindowInsets(padding)) {
                    navigation(route = TABS[0].graph, startDestination = "lists") {
                        composable("lists") { ListsScreen() }
                        composable("matrix") { MatrixScreen() }
                        composable("today") { TodayScreen() }
                        composable("habits") { HabitsScreen() }
                        composable("upcoming") { UpcomingScreen() }
                        composable("all") { AllScreen() }
                        composable("inbox") { InboxScreen() }
                        composable("done") { DoneScreen() }
                        composable("search") { SearchScreen() }
                        composable("project/{id}") { ProjectScreen(it.arguments?.getString("id").orEmpty()) }
                        composable("settings") { SettingsScreen() }
                        composable("approve") { ApproveScreen() }
                        activityPage(TABS[0])
                    }
                    navigation(route = TABS[1].graph, startDestination = "calendar") {
                        composable("calendar") { CalendarScreen() }
                        activityPage(TABS[1])
                    }
                    navigation(route = TABS[2].graph, startDestination = "focus") {
                        composable("focus") { FocusScreen() }
                        composable("stats") { StatsScreen() }
                        activityPage(TABS[2])
                    }
                }
            }
            // The timer and the sheets sit outside the pages, so their data would otherwise live in
            // the activity's store and outlast the account it belongs to.
            CompositionLocalProvider(LocalViewModelStoreOwner provides session) {
                if (zen && running != null) {
                    BackHandler { app.zen.value = false }
                    ZenScreen(running)
                }
                SheetHost()
            }
        }
    }
}

private fun NavGraphBuilder.activityPage(tab: Tab) = composable("${tab.graph}/activity") { ActivityScreen() }

/** Moves to another tab, keeping each tab's own stack of pages. */
private fun NavHostController.switchTab(tab: Tab) {
    navigate(tab.graph) {
        popUpTo(graph.findStartDestination().id) { saveState = true }
        launchSingleTop = true
        restoreState = true
    }
}

/** Opens a page by name: the matrix on a normal start, or what a debug launch asked for. */
private fun NavHostController.open(app: AppModel, route: String) {
    when (route) {
        "lists" -> Unit
        "calendar", "calendar-day" -> {
            app.calendarMode.value = if (route == "calendar") 1 else 0
            switchTab(TABS[1])
        }
        "focus" -> switchTab(TABS[2])
        "stats" -> {
            switchTab(TABS[2])
            navigate("stats")
        }
        "project" -> app.boot.value?.projects?.firstOrNull()?.let { navigate("project/${it.id}") }
        "activity" -> navigate("${TABS[0].graph}/activity")
        else -> navigate(route)
    }
}

/** The running timer above the navigation bar: the ring, the time left and what it is for. */
@Composable
private fun TimerStrip(onClick: () -> Unit) {
    val app = LocalApp.current
    val focus by app.focus.collectAsStateWithLifecycle()
    val f = focus ?: return
    val session = f.session ?: return
    val now = serverNow()
    val end = Times.instant(session.end).toEpochMilli()
    val over = f.state == "over"
    val color = when (f.state) {
        "rest" -> K.green
        "over" -> K.tomato
        else -> session.projectId?.let { projectColor(it) } ?: K.tint
    }
    Row(
        Modifier.fillMaxWidth().background(K.bg).padding(horizontal = 12.dp, vertical = 4.dp).height(48.dp).clip(RoundedCornerShape(24.dp))
            .background(K.fill).clickable(onClick = onClick).padding(start = 13.dp, end = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        RingMini(color, if (over) 1f else FocusMath.fractionLeft(end, session.plannedMinutes, now))
        Spacer(Modifier.width(9.dp))
        if (!over) {
            Text(FocusMath.mmss(FocusMath.secondsLeft(end, now)), fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = K.label)
            Spacer(Modifier.width(9.dp))
        }
        Text(
            when (f.state) {
                "rest" -> stringResource(R.string.focus_resting)
                "over" -> stringResource(R.string.focus_earned, f.tomatoesToday)
                else -> session.title.ifEmpty { stringResource(R.string.focus_free) }
            },
            fontSize = 15.sp, color = if (over) K.label else K.label2, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f),
        )
        Icon(KIcons.Right, null, tint = K.label3, modifier = Modifier.size(15.dp))
    }
}
