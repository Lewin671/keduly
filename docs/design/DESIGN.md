# Design

The clickable mockup is [mockup.html](mockup.html): one file, open it in a browser. Its data is
static sample data, and its interface text is Chinese, like the product's.

## Goal

Help one person manage their time and the things they have to do. The test: open it once a day
and know within a minute what today holds and whether it fits.

## Model

| Object | Meaning | Key fields |
|---|---|---|
| Project | A group of related things. It is both a list of items and a calendar colour | name, colour |
| Event | Something with a fixed time (a meeting, the dentist) | start, end, project |
| Item | Something to do that has no fixed time yet | project (none = inbox), estimate, deadline, important, status |
| Time block | The stretch of calendar an item was scheduled into | item, start, end |
| Focus session | A stretch of time actually spent working, usually on one item | item (none = free focus), start, end |
| Suggestion | A change an agent wants to make. It has no effect until accepted, one by one | change, reason, source |
| Activity | Every change that happened | who, when, what, why |

Items and events meet on the calendar: scheduling an item creates a time block, drawn on the same
timeline as events.

Projects are the backbone. Events and items belong to a project and take its colour on the
calendar, so the calendar shows at a glance where time goes. Items that are not scheduled still
live in their project's list. To a CalDAV client a project is a calendar of the same name, and a
time block is an ordinary event in it.

## Priority: important and urgent

Keduly uses the two axes of the Eisenhower matrix instead of a multi-level priority.

| Axis | Source | Rule |
|---|---|---|
| Important | Marked by the user | Two levels only: important or not. One switch |
| Urgent | Derived from the deadline, never stored | Overdue, or due today or tomorrow |

| Quadrant | Condition | Name in the interface |
|---|---|---|
| 1 | important, urgent | 马上做 (do now) |
| 2 | important, not urgent | 安排时间做 (schedule it); the heading says how many are not on the calendar yet |
| 3 | not important, urgent | 尽快处理掉 (get it out of the way) |
| 4 | neither | 有空再说 (when there is time) |

| Place | How it shows |
|---|---|
| Item row | An orange "!" before the title of important items |
| Item card | A switch to mark it important |
| Sidebar | The 四象限 list sorts every open item into the four boxes |
| Today | Important items come first. When the day cannot fit everything, the summary says how many important items to protect and how many others can wait |
| Agent scheduling | Important items get the uninterrupted slots first; unscheduled quadrant-2 items are proposed a time (not shown in the mockup) |
| CLI | `keduly item add ... --important` |

Orange means "important" and nothing else. Red is reserved for deadlines and the current time.

## Focus

Focus is a pomodoro timer and the third mode, next to items and calendar. Estimates and time
blocks say what was planned; tomatoes say what happened.

| Rule | Detail |
|---|---|
| Unit | A tomato (番茄): one work period run to its end, 25 minutes by default. Minutes are kept as well, for totals |
| Round | Four tomatoes make a round. A short rest (5 minutes) follows each tomato, a long rest (15 minutes) the fourth. The round is counted per day |
| Estimates | An item's estimate stays a duration; the interface also shows it as tomatoes, "2/4" = two earned of the four it comes to |
| One timer | One per account, kept on the server as a start time and a length, so the web app, the CLI and every device show the same countdown |
| No pause | A tomato is indivisible. Giving up (放弃) keeps the minutes spent but earns no tomato; under one minute counts as a slip and is dropped |
| Running out | The tomato is earned and the timer asks what next: 做完了 (tick the item), 再来一个, 休息. Unanswered, it stays there; nothing starts by itself |
| Switching | Starting on another item gives up the running tomato |
| Ticking the item | Ends its timer and keeps the minutes |
| Free focus | A tomato may belong to no item. It counts towards the day and the round, not towards any estimate |
| CalDAV | Sessions are never exported: they would flood the phone's calendar with 25-minute entries |

### The timer page

| State | What it shows |
|---|---|
| Idle | The sidebar lists today's open items (important first) with their tomatoes, then free focus; clicking one picks it. The page: "第 N 个番茄", a full dial reading 25:00, the picked item, the round of four tomato marks, 开始专注, and today's total |
| Working | The page takes the whole window: sidebar and toolbar disappear, the background is tinted with the project's colour, the dial drains. 放弃 and 做完了. 收起 at the top left returns to the app |
| Tomato earned | The dial closes in tomato red around a large tomato; one more mark in the round fills. 做完了, 再来一个, and the rest (short or long) as the main action |
| Resting | The same page in green, counting the rest down. 跳过休息 |

The dial follows Apple's Clock timer: one ring, large light figures, nothing else inside it.

### Elsewhere in the app

| Place | How it shows |
|---|---|
| Toolbar | While a timer runs and the timer page is not showing: a capsule with a small draining ring, the time left and the item. Clicking it returns to the timer page. Nothing when idle |
| Item row | A play button on hover (always visible on touch) starts a tomato and opens the timer page. The item being worked on shows the countdown instead |
| Item row, second line | The estimate, then the tomatoes: "1.5 小时 · 2/4"; once done, "用了 2 小时 5 分钟 · 5" |
| Item card | The estimate chip adds "已用 …"; a blue 开始专注 / 回到计时 chip |
| Today | The work still to schedule is the estimate minus the time already spent |
| Project page | The summary adds "本周已专注 …" |
| Calendar, day and week | Clicking an open time block offers 开始专注. Sessions are drawn as a thin line in the project's colour at the left edge of the day: the plan in blocks, what happened beside it |
| When time is up elsewhere | A HUD "完成第 N 个番茄" with 开始休息, a browser notification and a sound |
| Settings | 专注: the length of a tomato, of the rest, of the long rest and how often, the reminder |
| CLI | `keduly focus start <item>`, `status`, `stop`, and a log and statistics an agent can read to correct estimates |

### Statistics

Modelled on Screen Time in System Settings: figures in plain large type, one chart, then lists.

| Part | Contents |
|---|---|
| Figures | Today, the last 7 days, the daily average, and the streak of days with at least one tomato |
| Chart | One bar per day for the last 7 days, stacked by project colour, the count above each bar, a dashed line for the average. Today's label is red |
| 时间花在哪 | One row per project, most time first: tomatoes, time, and a thin bar relative to the top project |
| Records | Day by day, newest first: each session with its time, item, and a tomato or "未完成 · N 分钟". Two days up front, then "更早的记录" |

Tomato red is used only for tomato marks. Going over an estimate ("7/1") is not coloured.

## Principles

| Principle | In the interface |
|---|---|
| One timeline | Events and time blocks share the time axis and take their project's colour |
| Time is a budget | Items carry an estimate; Today states in one sentence how much free time is left and how much unscheduled work there is |
| Agents stay quiet | A suggestion only appears as a tentative entry on the calendar. There is no separate prompt area. Click it to read the reason, then accept or reject |
| Changes are traceable and reversible | The activity panel records the source and reason of every change, and each can be undone |
| Dangerous operations need consent | An agent's delete does not run; it waits in the activity panel for the user to allow or refuse |
| The interface does not explain itself | Headings do not carry explanatory subtitles. The screen must read on its own |

## Structure

Items, calendar and focus are three modes, switched at the top of the sidebar. Items and calendar are in effect a task manager and
Apple's Calendar in one window. Items come first and are what the app opens on (Today): managing
things to do is the centre of the product, and the calendar is the second view onto the same data. They are linked through projects and time. An item with a time
shows on the calendar in its project's colour, and a time block can be checked off right there.

| Mode | Sidebar | Main area |
|---|---|---|
| Items | Inbox, Today, Upcoming, Matrix, All, Done; then projects grouped by area, each with a progress ring | The selected list, in one centred column |
| Focus | Timer, Statistics; then today's open items to pick from | The timer page or statistics. See "Focus" |
| Calendar | A mini month; the project list, where a tick decides which projects the calendar shows | Day, week, month and year views |

The bell and the gear at the top right are shared by all modes.

| Button | Purpose |
|---|---|
| Bell | Activity panel: what is waiting (deletes that need consent, the number of tentative entries) and recent changes. The red badge counts what is waiting |
| Gear | Settings: account (with signing in on another device), agent tokens, system calendar (CalDAV), command line, appearance |

Creating: the toolbar "+" in calendar mode, the round blue "+" at the bottom right in items mode.

### Calendar views

| View | Contents |
|---|---|
| Day, week | A time axis. Events are tinted blocks with a bar on the left; time blocks carry a checkbox |
| Month | One cell per day listing that day's entries: a filled dot for an event, a hollow dot for a time block, a dashed dot for a tentative entry. At most 4 rows, then "还有 N 项"; all-day events and tentative entries are kept first |
| Year | Twelve small months. The depth of orange shows how busy each day is; today is a red dot. Clicking a month opens it |

On a phone there is no sidebar: a floating three-tab bar switches between items, calendar and focus, and
month cells show one dot per entry.

### Items mode

Calendar mode follows Apple's Calendar. Items mode deliberately does not follow Apple's Reminders;
its structure follows Things 3, with single ideas borrowed from Todoist and Sunsama.

| List | Contents |
|---|---|
| Inbox | Items not filed under a project yet |
| Today | One sentence on free time and unscheduled work; today's events in small grey type with a project-coloured bar; today's items, each with its project and estimate underneath; things for after work under "今晚" |
| Upcoming | Day by day from tomorrow: a large date, that day's events, the items planned or due that day |
| Matrix | Every open item in the four quadrants. See "Priority" |
| All | Every open item, one section per project; the project name opens the project |
| Done | Finished items |
| Project page | Progress ring and name; the project's notes; how many items are open and how many are not on the calendar; the project's next events; then the items, optionally under headings |

An item row:

| Element | Notes |
|---|---|
| Checkbox | A rounded square that fills and ticks when done. The checkbox on a time block is the same shape in the project's colour |
| Star | In a project page and in All, items planned for today carry a yellow star |
| Right side | The deadline with a small flag, red when due today; the scheduled time in grey; an agent's proposed time in a dashed box, "待定 14:00" |
| Opening it | The row expands in place into a card: notes, the agent's suggestion with its reason (accept or reject), time, estimate, deadline, project |

## Signing in on another device

Typing a password on every new device is tedious, so a device that is signed in can vouch for one
that is not. There are two directions, because only one of the two devices needs a camera, and
both use the system camera: the QR code is a link to the server.

| Direction | Where it starts | What the other device does |
|---|---|---|
| The new device shows the code | "扫码登录" on the sign-in screen: a QR code with a 4-digit number under it | The signed-in phone scans it, sees which browser and system is asking, types the number and taps "允许登录" |
| The signed-in device shows the code | "在其他设备登录…" in Settings, under the account | The new device scans it, sees whose account it is, and taps "登录" |

Rules that the design must keep:

- Nothing happens on a scan alone. Both screens ask first, and say which account is involved.
- The number is typed, not compared: it proves the person approving can see the other screen. A
  code or link forwarded by someone else arrives without it. The approval screen says so in plain
  words and offers "拒绝" as prominently as it can without being the default.
- A code lives 2 minutes and works once. An expired one is replaced by a button, never refreshed
  silently, so a screen left open does not keep a live code on show.
- The code in Settings is a temporary password and is described as one. Closing the row withdraws it.
- The QR code is always dark on white, in both themes.

| Case | Handling |
|---|---|
| The scanning phone is not signed in | It asks for the password first, then continues to the approval screen |
| A wrong number | "数字不对"; the field clears. The third wrong number ends the request |
| Expired, used, refused or withdrawn | "二维码已失效" and what to do on the other device |
| The scanning device already has that account | It says so and leaves the code unused |
| The scanning device has another account | It warns that the account will be replaced |
| The browser and system are not recognised | "一台设备想登录…", with no name: the asking device cannot put its own words there |

The mockup's demo switch has a "换设备登录" row that shows each of these screens.

## Edge cases

The mockup has a "设计稿演示数据" switch at the bottom of the sidebar (for the mockup only, not a
product feature) that swaps the sample data between ordinary, crowded and empty.

### Long text

| Case | Handling |
|---|---|
| Long item title | Two lines in a list, then an ellipsis; shown in full in the card |
| Long item notes | Only in the card, wrapped in full |
| Long project name | One line with an ellipsis in both sidebars, full name on hover; the project page title may wrap |
| Long event title | Up to three lines when the block is an hour or taller, otherwise one line; one line in month view; full title on hover |

### Large numbers

| Case | Handling |
|---|---|
| A project with dozens of items | Headings fold, and show their open count; items finished earlier collapse into "显示 N 件已完成" at the bottom |
| An item just checked off | Stays in place, shown as done, so the tick can be undone; it is folded away on the next visit |
| Many projects | The sidebar groups them by area and scrolls |
| A large inbox or Today count | Counts above 99 show as "99+" |
| Many events today | Today lists the first 5, then "还有 N 个日程，在日历里查看" |
| A crowded day in month view | 4 rows per cell, then "还有 N 项" |
| Many suggestions | The bell panel shows the count and offers "全部接受", which can be undone in one step |

### Nothing at all

| Case | Handling |
|---|---|
| An empty Today, Upcoming, All, Done, Inbox or project | Each says what it is for and what to do next. Never a blank screen |
| No projects yet | The sidebar shows the lists and "新建项目"; the calendar sidebar says "还没有项目" |
| No activity | The panel says so, and the bell has no badge |

### Unusual times

| Case | Handling |
|---|---|
| Overlapping events | A cluster of overlaps shares the column side by side, in as many lanes as it needs. In week view, three lanes or more show the title only |
| Very short events (15 minutes) | A minimum readable height, with title and time on one line |
| All-day and multi-day events | Day view has an all-day strip on top; week view has a row under the dates in which an event spans the days it covers; month view shows a coloured bar on each day |
| Early-morning and late-night events | The time axis covers all 24 hours and scrolls, opening at about 07:30 |
| Not enough time today | Today's summary says "排不下" and what to protect; when working hours are full it says so. Overlapping events are not counted twice |

### Unusual states

| Case | Handling |
|---|---|
| Overdue item | A red "逾期 N 天" on the right; Today lists overdue items first, under their own heading |
| Item without an estimate or a project | Both allowed. Without a project it is in the inbox |

### Focus

| Case | Handling |
|---|---|
| Long item title | Two lines under the dial; one line with an ellipsis in the sidebar and the toolbar capsule; on a phone the capsule shows the ring and the time only |
| Many items today | The sidebar list scrolls |
| Far over the estimate, or no estimate | "7/1", or just the count. No colour |
| A day without tomatoes | An empty column in the chart; the streak ends there |
| No sessions at all | Statistics say so and what to do; the timer page still works |
| No items at all | Free focus is the only choice and is picked |
| Only given-up sessions on an item | The row says "已用 12 分钟" instead of a tomato count |
| The page was closed when time ran out | The tomato was still earned on the server at its planned end; the page shows the "what next" state on the next visit |

### Not designed yet

| Case | Intended handling |
|---|---|
| Sync failure or offline | A status and a retry entry at the bottom of the sidebar; local changes are kept |
| A stale suggestion (its time has passed or the slot was taken) | Marked invalid and no longer directly acceptable |
| Multi-day events in month view | Drawn as one segment per day today; ideally one continuous bar |
| Overlapping events on a phone | The lanes get very narrow; needs its own design |
| Time up on a phone | A web page cannot ring once it is closed; a reliable reminder needs push notifications |
| Correcting a session | Deleting or shortening a wrong record (forgot to stop before leaving) |
| Statistics beyond 7 days | Weeks, months, and a comparison with the previous period |

## Loading on demand

Data grows for as long as the product is used, so the interface must not fetch everything at
once. On opening, fetch only what is on screen; fetch the rest when the user gets there.

Fetched on opening:

| What | Why |
|---|---|
| Today's and this week's items and events | The first screen needs them |
| The project list and the counts in the sidebar | The server computes the counts; they are not derived by fetching every item |
| Each project's progress | Only a ratio is needed |

Fetched on arrival:

| Place | How | Batch |
|---|---|---|
| Done | Grouped by month; scrolling to the bottom fetches the next month back, with placeholder rows meanwhile; the end says "已经到最早的一件了". The heading shows the total | one month |
| All | The first 5 items per project, then "再显示" | 15 |
| Matrix | The first 6 items per quadrant | 18 |
| Finished items in a project | Collapsed; fetched when "显示 N 件已完成" is clicked | 10, then 30 |
| Inbox | The first 20 | 60 |
| Upcoming | The current month, then a button for the next | one month |
| Activity | The most recent few, then "更早的动态" | one batch |
| Calendar | Only the visible range, with the neighbouring ranges prefetched so paging feels instant; the year view fetches a count per day, not the events | one range |
| Search | Runs on the server over everything, never over what happens to be loaded | one page |

Lists that only grow and that people scroll through (Done) load on scroll. Everything else uses an
explicit button, so the page never gets longer when the user does not expect it.

## Visual language

Modelled on macOS Calendar and System Settings, so that it does not look out of place next to
Apple's own apps.

| Part | Reference | Approach |
|---|---|---|
| Sidebar | Calendar, Things 3 | A floating translucent panel with an 18 px radius; the mode switch on top |
| Titles | Calendar | A bold date with the weekday in light grey, tight letter spacing |
| Toolbar | Calendar | Capsule controls: previous and next around a red "今天" |
| Time axis | Calendar | Events are tinted with a rounded bar on the left and text in a darker shade of the same colour. The current time is a red line with a red capsule; today's date is a red circle; weekend columns are slightly grey |
| Items | Things 3 | See "Items mode". The sidebar is a quiet text list, not coloured tiles |
| Activity, Settings | Calendar's inbox, System Settings | Activity is a translucent popover under its toolbar button. Settings are rounded grouped lists with a coloured square icon on each row and a line of small print under each group |
| Popovers | System popovers | Translucent, 14 px radius, a springy scale-in, capsule buttons |
| Phone | iOS | A floating translucent capsule tab bar with two entries |

| Topic | Rule |
|---|---|
| Type | System fonts (SF Pro, PingFang). Titles 28 px bold, body 15 px |
| Colour | Apple's system colours with their dark-mode counterparts |
| Project colours | Blue, indigo, orange, teal, green, pink, purple, brown. One per project; used for calendar blocks, checkboxes and list titles |
| Blue | Also the colour of every clickable action |
| Red | Only the current time, today, and deadlines |
| Orange | Only "important" |
| Tentative (agent suggestion) | Same position and size as a normal entry, with a dashed outline, a very light fill and the label "待定" |
| An event proposed to move away | Faded and struck through at its old place; tentative at the new one |
| Translucency | Only the sidebar, popovers, the HUD and the phone tab bar. Never the content area |

Light and dark are both supported, following the system by default.

## Directions that were tried and rejected

| Version | Approach | Feedback |
|---|---|---|
| 1 | Rounded cards, a purple agent-suggestion card, coloured tags, a subtitle under every heading | Looks AI-generated |
| 2 | A paper planner: serif headings, a ruler, a strip of cells, hatching, red stamps | Harder to use, and fussy |
| 3 | White background, hairlines, a plain sidebar | Nowhere near Apple's design; none of its character |
| 4 | Apple style, but five pages (today, week, tasks, activity, access) and sidebar tiles that each led to a different kind of page | Has the Apple character, but the pages combine oddly |
| 5–6 | One screen: tasks (later with projects) on the left, calendar on the right | The two halves still sit oddly together; items and calendar should be separate |
| 7, items mode | Copied Apple's Reminders: coloured tiles, round checkboxes, flat project lists | Reminders itself is not good; no need to copy it |
