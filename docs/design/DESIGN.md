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

Items and calendar are two modes, switched at the top of the sidebar: in effect a task manager and
Apple's Calendar in one window. Items come first and are what the app opens on (Today): managing
things to do is the centre of the product, and the calendar is the second view onto the same data. They are linked through projects and time. An item with a time
shows on the calendar in its project's colour, and a time block can be checked off right there.

| Mode | Sidebar | Main area |
|---|---|---|
| Items | Inbox, Today, Upcoming, Matrix, All, Done; then projects grouped by area, each with a progress ring | The selected list, in one centred column |
| Calendar | A mini month; the project list, where a tick decides which projects the calendar shows | Day, week, month and year views |

The bell and the gear at the top right are shared by both modes.

| Button | Purpose |
|---|---|
| Bell | Activity panel: what is waiting (deletes that need consent, the number of tentative entries) and recent changes. The red badge counts what is waiting |
| Gear | Settings: account, agent tokens, system calendar (CalDAV), command line, appearance |

Creating: the toolbar "+" in calendar mode, the round blue "+" at the bottom right in items mode.

### Calendar views

| View | Contents |
|---|---|
| Day, week | A time axis. Events are tinted blocks with a bar on the left; time blocks carry a checkbox |
| Month | One cell per day listing that day's entries: a filled dot for an event, a hollow dot for a time block, a dashed dot for a tentative entry. At most 4 rows, then "还有 N 项"; all-day events and tentative entries are kept first |
| Year | Twelve small months. The depth of orange shows how busy each day is; today is a red dot. Clicking a month opens it |

On a phone there is no sidebar: a floating two-tab bar switches between calendar and items, and
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

### Not designed yet

| Case | Intended handling |
|---|---|
| Sync failure or offline | A status and a retry entry at the bottom of the sidebar; local changes are kept |
| A stale suggestion (its time has passed or the slot was taken) | Marked invalid and no longer directly acceptable |
| Multi-day events in month view | Drawn as one segment per day today; ideally one continuous bar |
| Overlapping events on a phone | The lanes get very narrow; needs its own design |

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
