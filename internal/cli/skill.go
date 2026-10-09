package cli

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Lewin671/keduly/skills"
)

const skillName = "keduly"

// skillInstall writes the skill built into this binary into a skills
// directory, so the skill always describes the CLI that installed it.
func (a *app) skillInstall(args []string) error {
	fs := a.flags("skill install", false)
	claude := fs.Bool("claude", false, "install for Claude Code, which reads .claude/skills instead of .agents/skills")
	project := fs.Bool("project", false, "install into the current directory's project instead of the home directory")
	dir := fs.String("dir", "", "install into this skills directory instead")
	if _, err := a.parseN(fs, args, 0, "[--claude] [--project] | --dir DIR"); err != nil {
		return err
	}
	if *dir != "" && (*claude || *project) {
		return usagef("--dir cannot be combined with --claude or --project")
	}
	if *dir == "" {
		root, err := a.agentDir(*claude, *project)
		if err != nil {
			return err
		}
		*dir = filepath.Join(root, "skills")
	}
	target := filepath.Join(*dir, skillName)
	// A link is someone's working copy of the skill; writing through it would change that copy.
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return a.skillResult(target, false, "%s 是一个符号链接，保持不变\n")
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	if err := os.CopyFS(target, mustSub(skills.FS, skillName)); err != nil {
		return err
	}
	return a.skillResult(target, true, "Skill 已安装到 %s\n")
}

func (a *app) skillResult(path string, installed bool, format string) error {
	if a.json {
		data, _ := json.Marshal(map[string]any{"path": path, "installed": installed})
		a.printf("%s\n", data)
		return nil
	}
	a.printf(format, path)
	return nil
}

// agentDir is the directory that holds an agent's skills directory: .agents,
// the convention agents share, or .claude, in the project or the home directory.
func (a *app) agentDir(claude, project bool) (string, error) {
	name := ".agents"
	if claude {
		name = ".claude"
	}
	if project {
		return name, nil
	}
	if dir := a.env.Getenv("CLAUDE_CONFIG_DIR"); claude && dir != "" {
		return dir, nil
	}
	home := a.env.Getenv("HOME")
	if home == "" {
		var err error
		if home, err = os.UserHomeDir(); err != nil {
			return "", fmt.Errorf("cannot find the home directory; pass --dir: %v", err)
		}
	}
	return filepath.Join(home, name), nil
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
