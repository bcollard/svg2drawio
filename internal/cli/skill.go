package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// claudeSkillDir returns Claude Code's user skills directory for svg2drawio.
func claudeSkillDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "skills", "svg2drawio")
}

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the svg2drawio Agent Skill for AI coding tools",
		Long: `Manage the svg2drawio Agent Skill — a portable capability description that
teaches AI coding tools (e.g. Claude Code) how to turn SVGs into editable
draw.io diagrams without being told each session.

The skill is SKILL.md from the repository, embedded in this binary.`,
	}
	cmd.AddCommand(newSkillInstallCmd(), newSkillPathCmd())
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	var (
		claude bool
		stdout bool
		force  bool
		dir    string
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the svg2drawio Agent Skill into an AI coding tool",
		Long: `Install the svg2drawio Agent Skill (SKILL.md) into your AI coding tool's
skills directory.

By default it installs into Claude Code's user skills directory:

  ~/.claude/skills/svg2drawio/SKILL.md

Use --print to write the skill to stdout instead, or --dir to install it
somewhere else (a project's .claude/skills directory, for example).`,
		Example: `  svg2drawio skill install
  svg2drawio skill install --force
  svg2drawio skill install --dir .claude/skills/svg2drawio
  svg2drawio skill install --print > SKILL.md`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if skillMD == "" {
				return errors.New("this build has no embedded skill")
			}
			if stdout {
				fmt.Fprint(cmd.OutOrStdout(), skillMD)
				return nil
			}
			target := dir
			if target == "" {
				if !claude {
					return errors.New("no install target selected (pass --claude, --dir or --print)")
				}
				target = claudeSkillDir()
			}
			dest, err := installSkill(target, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", dest)
			return nil
		},
	}
	cmd.Flags().BoolVar(&claude, "claude", true, "Install into Claude Code's user skills directory")
	cmd.Flags().BoolVar(&stdout, "print", false, "Write the skill to stdout instead of installing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite an existing installed skill")
	cmd.Flags().StringVar(&dir, "dir", "", "Install into this directory instead")
	return cmd
}

func newSkillPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print where the Agent Skill is installed for Claude Code",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), filepath.Join(claudeSkillDir(), "SKILL.md"))
			return nil
		},
	}
}

// installSkill writes SKILL.md into dir, refusing to clobber an existing file
// unless force is set.
func installSkill(dir string, force bool) (string, error) {
	dest := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(dest); err == nil && !force {
		return "", fmt.Errorf("%s already exists (use --force to overwrite)", dest)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, []byte(skillMD), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}
