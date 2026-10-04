// Package api holds the JSON shapes of the HTTP API described in docs/api.md.
// The server encodes them and the CLI decodes them.
package api

import "encoding/json"

type Actor struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Timezone  string `json:"timezone"`
	WorkStart string `json:"work_start"`
	WorkEnd   string `json:"work_end"`
	CreatedAt string `json:"created_at"`
}

type Area struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type Project struct {
	ID        string  `json:"id"`
	AreaID    *string `json:"area_id"`
	Name      string  `json:"name"`
	Color     string  `json:"color"`
	Notes     string  `json:"notes"`
	Position  int     `json:"position"`
	Archived  bool    `json:"archived"`
	OpenCount int     `json:"open_count"`
	DoneCount int     `json:"done_count"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type Heading struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
}

type Block struct {
	EventID string `json:"event_id"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

type ItemSuggestion struct {
	ID     string `json:"id"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Reason string `json:"reason"`
	Actor  Actor  `json:"actor"`
}

type Item struct {
	ID              string          `json:"id"`
	ProjectID       *string         `json:"project_id"`
	HeadingID       *string         `json:"heading_id"`
	Title           string          `json:"title"`
	Notes           string          `json:"notes"`
	EstimateMinutes *int            `json:"estimate_minutes"`
	PlannedDate     *string         `json:"planned_date"`
	Evening         bool            `json:"evening"`
	DueDate         *string         `json:"due_date"`
	DueTime         *string         `json:"due_time"`
	Important       bool            `json:"important"`
	Status          string          `json:"status"`
	CompletedAt     *string         `json:"completed_at"`
	Position        int             `json:"position"`
	Block           *Block          `json:"block"`
	Suggestion      *ItemSuggestion `json:"suggestion"`
	CreatedBy       Actor           `json:"created_by"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

type Event struct {
	ID           string  `json:"id"`
	ProjectID    *string `json:"project_id"`
	ItemID       *string `json:"item_id"`
	Title        string  `json:"title"`
	Notes        string  `json:"notes"`
	Location     string  `json:"location"`
	AllDay       bool    `json:"all_day"`
	Start        *string `json:"start"`
	End          *string `json:"end"`
	StartDate    *string `json:"start_date"`
	EndDate      *string `json:"end_date"`
	RRule        *string `json:"rrule"`
	Recurring    bool    `json:"recurring"`
	Instance     *string `json:"instance"`
	Status       string  `json:"status"`
	SuggestionID *string `json:"suggestion_id"`
	ItemDone     *bool   `json:"item_done"`
	Readonly     bool    `json:"readonly"`
	CreatedBy    Actor   `json:"created_by"`
	UpdatedAt    string  `json:"updated_at"`
}

type Suggestion struct {
	ID        string          `json:"id"`
	Status    string          `json:"status"`
	Kind      string          `json:"kind"`
	Actor     Actor           `json:"actor"`
	Reason    string          `json:"reason"`
	Title     string          `json:"title"`
	ItemID    *string         `json:"item_id"`
	EventID   *string         `json:"event_id"`
	Start     *string         `json:"start"`
	End       *string         `json:"end"`
	Item      json.RawMessage `json:"item"`
	Event     json.RawMessage `json:"event"`
	CreatedAt string          `json:"created_at"`
	DecidedAt *string         `json:"decided_at"`
}

type Activity struct {
	ID        string  `json:"id"`
	Actor     Actor   `json:"actor"`
	Action    string  `json:"action"`
	Summary   string  `json:"summary"`
	Reason    *string `json:"reason"`
	Undoable  bool    `json:"undoable"`
	Undone    bool    `json:"undone"`
	CreatedAt string  `json:"created_at"`
}

type Token struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	Scope         string  `json:"scope"`
	ConfirmDelete bool    `json:"confirm_delete"`
	LastUsedAt    *string `json:"last_used_at"`
	CreatedAt     string  `json:"created_at"`
	Token         string  `json:"token,omitempty"`
}

type Counts struct {
	Inbox   int `json:"inbox"`
	Today   int `json:"today"`
	Pending int `json:"pending"`
}

type Slot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Day struct {
	Date   string  `json:"date"`
	Events []Event `json:"events"`
	Items  []Item  `json:"items"`
}

type Quadrant struct {
	Total     int    `json:"total"`
	Unplanned int    `json:"unplanned"`
	Items     []Item `json:"items"`
}

type OverviewProject struct {
	ProjectID string `json:"project_id"`
	Total     int    `json:"total"`
	Items     []Item `json:"items"`
}

type Today struct {
	Date             string  `json:"date"`
	Items            []Item  `json:"items"`
	Overdue          []Item  `json:"overdue"`
	Events           []Event `json:"events"`
	FreeMinutes      int     `json:"free_minutes"`
	UnplannedMinutes int     `json:"unplanned_minutes"`
}

type ProjectDetail struct {
	Project        Project   `json:"project"`
	Headings       []Heading `json:"headings"`
	UpcomingEvents []Event   `json:"upcoming_events"`
	UnplannedCount int       `json:"unplanned_count"`
}

type Bootstrap struct {
	User     User      `json:"user"`
	Areas    []Area    `json:"areas"`
	Projects []Project `json:"projects"`
	Headings []Heading `json:"headings"`
	Counts   Counts    `json:"counts"`
	Revision int64     `json:"revision"`
}

type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
