// Package cli is the command-line client of the JSON API.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lewin671/keduly/internal/api"
)

// Env is what the CLI needs from its surroundings; tests supply their own.
type Env struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
	Getenv  func(string) string
	Now     func() time.Time
	HTTP    *http.Client
}

const usage = `keduly - calendar and task manager with a first-class interface for agents

Usage:
  keduly serve [--addr 127.0.0.1:8080] [--data ./data]    run the server
  keduly login --server URL [--token TOKEN]               save server and token
  keduly whoami                                           show the signed-in account
  keduly agenda [--date D] [--days N]                     events, time blocks and planned items per day
  keduly free --date D --duration 60m                     free slots within working hours
  keduly item add TITLE [--project P] [--when D] [--due D] [--estimate 30m] [--important] [--notes ...]
  keduly item list [--view today|inbox|upcoming|all|matrix|done] [--project P] [--limit N]
  keduly item show|done|reopen|rm|unschedule ID
  keduly item edit ID [flags]
  keduly item schedule ID --start T (--end T | --duration 1h)
  keduly event add TITLE --start T (--end T | --duration 1h) [--all-day] [--project P]
  keduly event list [--from D] [--to D]
  keduly event edit ID [flags] | move ID --start T | rm ID
  keduly project list | add NAME [--color C] [--area A]
  keduly suggest schedule ITEM_ID --start T --duration 1h --reason "..."
  keduly suggest add-item TITLE [item flags] [--start T --duration D] --reason "..."
  keduly suggest add-event TITLE --start T --duration D --reason "..."
  keduly suggest move EVENT_ID --start T [--duration D] --reason "..."
  keduly suggest list
  keduly activity [--limit N]
  keduly undo [ACTIVITY_ID]
  keduly version

Every client command accepts --json. Commands that change something accept
--dry-run and --reason "...". IDs may be shortened to a unique prefix of at
least 4 characters.

Dates: today, tomorrow, a weekday name, YYYY-MM-DD. Times: HH:MM (today),
"YYYY-MM-DD HH:MM", "tomorrow 14:00". Durations: 90m, 1h30m, 2h.
All dates and times are in the account's time zone.

Configuration: $XDG_CONFIG_HOME/keduly/config.json, or KEDULY_SERVER and KEDULY_TOKEN.
`

type config struct {
	Server string `json:"server"`
	Token  string `json:"token"`
}

type app struct {
	env    Env
	cfg    config
	json   bool
	dryRun bool
	reason string
	me     *meResponse
	loc    *time.Location
}

type meResponse struct {
	User  api.User  `json:"user"`
	Actor api.Actor `json:"actor"`
}

// Run executes one CLI invocation and returns the process exit code.
func Run(args []string, env Env) int {
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.HTTP == nil {
		env.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	a := &app{env: env}
	err := a.dispatch(args)
	if err == nil {
		return 0
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintln(env.Stderr, "keduly:", err)
	var u usageError
	if errors.As(err, &u) {
		return 2
	}
	return 1
}

type usageError struct{ msg string }

func (u usageError) Error() string { return u.msg }

func usagef(format string, a ...any) error { return usageError{fmt.Sprintf(format, a...)} }

func (a *app) dispatch(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(a.env.Stdout, usage)
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version", "--version":
		fmt.Fprintln(a.env.Stdout, "keduly", a.env.Version)
		return nil
	case "login":
		return a.login(rest)
	}
	if err := a.loadConfig(); err != nil {
		return err
	}
	switch cmd {
	case "whoami":
		return a.whoami(rest)
	case "agenda":
		return a.agenda(rest)
	case "free":
		return a.free(rest)
	case "item":
		return a.sub(rest, "item", map[string]func([]string) error{
			"add": a.itemAdd, "list": a.itemList, "show": a.itemShow, "edit": a.itemEdit,
			"done": a.itemStatus("done"), "reopen": a.itemStatus("open"), "rm": a.itemRemove,
			"schedule": a.itemSchedule, "unschedule": a.itemUnschedule,
		})
	case "event":
		return a.sub(rest, "event", map[string]func([]string) error{
			"add": a.eventAdd, "list": a.eventList, "edit": a.eventEdit, "move": a.eventEdit, "rm": a.eventRemove,
		})
	case "project":
		return a.sub(rest, "project", map[string]func([]string) error{"list": a.projectList, "add": a.projectAdd})
	case "suggest":
		return a.sub(rest, "suggest", map[string]func([]string) error{
			"schedule": a.suggestSchedule, "add-item": a.suggestAddItem, "add-event": a.suggestAddEvent,
			"move": a.suggestMove, "list": a.suggestList,
		})
	case "activity":
		return a.activity(rest)
	case "undo":
		return a.undo(rest)
	}
	return usagef("unknown command %q; run `keduly help`", cmd)
}

func (a *app) sub(args []string, group string, cmds map[string]func([]string) error) error {
	if len(args) == 0 {
		return usagef("missing subcommand; run `keduly help`")
	}
	fn, ok := cmds[args[0]]
	if !ok {
		return usagef("unknown command %q; run `keduly help`", group+" "+args[0])
	}
	return fn(args[1:])
}

func (a *app) configPath() (string, error) {
	dir := a.env.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home := a.env.Getenv("HOME")
		if home == "" {
			var err error
			if home, err = os.UserHomeDir(); err != nil {
				return "", err
			}
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "keduly", "config.json"), nil
}

func (a *app) loadConfig() error {
	if path, err := a.configPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			if err := json.Unmarshal(data, &a.cfg); err != nil {
				return fmt.Errorf("%s is not valid JSON: %v", path, err)
			}
		}
	}
	if v := a.env.Getenv("KEDULY_SERVER"); v != "" {
		a.cfg.Server = v
	}
	if v := a.env.Getenv("KEDULY_TOKEN"); v != "" {
		a.cfg.Token = v
	}
	if a.cfg.Server == "" || a.cfg.Token == "" {
		return errors.New("not signed in; run `keduly login --server URL` or set KEDULY_SERVER and KEDULY_TOKEN")
	}
	return nil
}

func (a *app) saveConfig() (string, error) {
	path, err := a.configPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	data, _ := json.MarshalIndent(a.cfg, "", "  ")
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, os.Chmod(path, 0o600)
}

// flags returns a flag set with the options every command shares.
func (a *app) flags(name string, mutating bool) *flag.FlagSet {
	fs := flag.NewFlagSet("keduly "+name, flag.ContinueOnError)
	fs.SetOutput(a.env.Stderr)
	fs.BoolVar(&a.json, "json", false, "print the API response as JSON")
	if mutating {
		fs.BoolVar(&a.dryRun, "dry-run", false, "validate and show the result without writing")
		fs.StringVar(&a.reason, "reason", "", "why this change is made; shown in the activity log")
	}
	return fs
}

// parse accepts flags before, between and after positional arguments.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			return nil, usageError{err.Error()}
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// parseN parses and insists on exactly n positional arguments.
func parseN(fs *flag.FlagSet, args []string, n int, what string) ([]string, error) {
	pos, err := parse(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != n {
		return nil, usagef("usage: %s %s", fs.Name(), what)
	}
	return pos, nil
}

func given(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("%s (%s)", e.Message, e.Code) }

// request calls the API and returns the response body and status.
func (a *app) request(method, path string, query url.Values, body map[string]any) ([]byte, int, error) {
	if method != http.MethodGet {
		if a.dryRun {
			if query == nil {
				query = url.Values{}
			}
			query.Set("dry_run", "1")
		}
		if a.reason != "" {
			if body == nil {
				body = map[string]any{}
			}
			body["reason"] = a.reason
		}
	}
	target := strings.TrimRight(a.cfg.Server, "/") + "/api/v1" + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, target, payload)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.env.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot reach %s: %v", a.cfg.Server, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		var e api.ErrorBody
		if json.Unmarshal(data, &e) != nil || e.Error.Code == "" {
			return nil, resp.StatusCode, fmt.Errorf("the server answered %s", resp.Status)
		}
		return nil, resp.StatusCode, &apiError{resp.StatusCode, e.Error.Code, e.Error.Message}
	}
	return data, resp.StatusCode, nil
}

// send calls the API. With --json it prints the response and reports
// printed=true; otherwise it decodes the response into out.
func (a *app) send(method, path string, query url.Values, body map[string]any, out any) (printed bool, status int, err error) {
	data, status, err := a.request(method, path, query, body)
	if err != nil {
		return false, status, err
	}
	if a.json {
		if len(bytes.TrimSpace(data)) == 0 {
			data = []byte("{}")
		}
		fmt.Fprintln(a.env.Stdout, strings.TrimSpace(string(data)))
		return true, status, nil
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return false, status, fmt.Errorf("unexpected response: %v", err)
		}
	}
	return false, status, nil
}

// get fetches into out without ever printing, for lookups a command needs.
func (a *app) get(path string, query url.Values, out any) error {
	data, _, err := a.request(http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func (a *app) printf(format string, args ...any) {
	fmt.Fprintf(a.env.Stdout, format, args...)
}

// account loads the signed-in user once; its time zone drives date parsing.
func (a *app) account() (*meResponse, error) {
	if a.me != nil {
		return a.me, nil
	}
	var me meResponse
	if err := a.get("/me", nil, &me); err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(me.User.Timezone)
	if err != nil {
		loc = time.UTC
	}
	a.me, a.loc = &me, loc
	return a.me, nil
}

func (a *app) zone() (*time.Location, error) {
	if _, err := a.account(); err != nil {
		return nil, err
	}
	return a.loc, nil
}

// dryNote marks output that came from a dry run.
func (a *app) dryNote() string {
	if a.dryRun {
		return "（试运行，未写入）"
	}
	return ""
}
