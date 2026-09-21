package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const miniSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20">
	<rect x="1" y="1" width="8" height="8" fill="#ff0000"/></svg>`

func writeSVG(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(miniSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// run executes the root command with the given arguments and returns stdout.
func run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := tryRun(args...)
	if err != nil {
		t.Fatalf("run(%v): %v", args, err)
	}
	return out
}

func tryRun(args ...string) (string, error) {
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestRootConvertsWithoutASubcommand(t *testing.T) {
	dir := t.TempDir()
	in := writeSVG(t, dir, "icon.svg")

	run(t, "-q", in)

	data, err := os.ReadFile(filepath.Join(dir, "icon.drawio"))
	if err != nil {
		t.Fatalf("expected icon.drawio next to the input: %v", err)
	}
	if !strings.Contains(string(data), "<mxfile") || !strings.Contains(string(data), "fillColor=#ff0000") {
		t.Errorf("unexpected output:\n%s", data)
	}
}

func TestConvertSubcommandMatchesRoot(t *testing.T) {
	dir := t.TempDir()
	in := writeSVG(t, dir, "icon.svg")
	out := filepath.Join(dir, "explicit.drawio")

	run(t, "convert", "-q", "-o", out, in)

	if _, err := os.Stat(out); err != nil {
		t.Errorf("convert did not write %s: %v", out, err)
	}
}

func TestRootWithoutArgumentsPrintsHelp(t *testing.T) {
	out := run(t)
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("expected help output, got:\n%s", out)
	}
}

func TestStdoutOutput(t *testing.T) {
	dir := t.TempDir()
	in := writeSVG(t, dir, "icon.svg")

	out := run(t, "-o", "-", in)

	if !strings.HasPrefix(out, `<?xml version="1.0"`) || !strings.Contains(out, "<mxfile") {
		t.Errorf("stdout output looks wrong:\n%s", out)
	}
}

func TestDirectoryInputMirrorsIntoOutputDir(t *testing.T) {
	src := t.TempDir()
	writeSVG(t, src, "a.svg")
	writeSVG(t, src, "nested/b.svg")
	dst := filepath.Join(t.TempDir(), "out")

	run(t, "-q", "-o", dst, src)

	for _, name := range []string{"a.drawio", "b.drawio"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Errorf("expected %s in the output directory: %v", name, err)
		}
	}
}

func TestSeveralInputsToOneFileBecomePages(t *testing.T) {
	dir := t.TempDir()
	a := writeSVG(t, dir, "a.svg")
	b := writeSVG(t, dir, "b.svg")
	out := filepath.Join(dir, "both.drawio")

	run(t, "-q", "-o", out, a, b)

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "<diagram") != 2 {
		t.Errorf("expected two pages:\n%s", data)
	}
	if !strings.Contains(string(data), `name="a"`) || !strings.Contains(string(data), `name="b"`) {
		t.Errorf("pages are not named after the inputs:\n%s", data)
	}
}

func TestStencilCommandWritesALibrary(t *testing.T) {
	dir := t.TempDir()
	a := writeSVG(t, dir, "one.svg")
	b := writeSVG(t, dir, "two.svg")
	out := filepath.Join(dir, "lib.xml")

	run(t, "stencil", "-q", "--name", "icons", "-o", out, a, b)

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `<shapes name="icons">`) {
		t.Errorf("missing library root:\n%s", s)
	}
	if !strings.Contains(s, `name="icons.one"`) || !strings.Contains(s, `name="icons.two"`) {
		t.Errorf("shapes are not named after the inputs:\n%s", s)
	}
}

func TestScaleFlagIsApplied(t *testing.T) {
	dir := t.TempDir()
	in := writeSVG(t, dir, "a.svg")
	out := filepath.Join(dir, "scaled.drawio")

	run(t, "-q", "--scale", "2", "-o", out, in)

	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), `width="16"`) {
		t.Errorf("scale not applied:\n%s", data)
	}
}

func TestMissingInputIsAnError(t *testing.T) {
	if _, err := tryRun("does-not-exist.svg"); err == nil {
		t.Error("expected an error for a missing input")
	}
}

func TestDirectoryWithoutSVGsIsAnError(t *testing.T) {
	if _, err := tryRun(t.TempDir()); err == nil {
		t.Error("expected an error when no .svg files are found")
	}
}

func TestVersionCommandPrintsBuildSignature(t *testing.T) {
	SetBuildInfo("v1.2.3", "abc1234", "2026-01-01T00:00:00Z")
	t.Cleanup(func() { SetBuildInfo("dev", "none", "unknown") })

	out := run(t, "version")
	if !strings.Contains(out, "svg2drawio v1.2.3 (abc1234, 2026-01-01T00:00:00Z)") {
		t.Errorf("version output = %q", out)
	}
	if !strings.Contains(out, "platform:") {
		t.Errorf("version output is missing the platform line: %q", out)
	}

	short := run(t, "version", "--short")
	if strings.TrimSpace(short) != "v1.2.3" {
		t.Errorf("short version = %q", short)
	}
}

func TestSkillInstallWritesTheEmbeddedSkill(t *testing.T) {
	SetSkill("---\nname: svg2drawio\n---\nbody\n")
	t.Cleanup(func() { SetSkill("") })
	dir := filepath.Join(t.TempDir(), "skills", "svg2drawio")

	run(t, "skill", "install", "--dir", dir)

	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("skill not installed: %v", err)
	}
	if !strings.Contains(string(data), "name: svg2drawio") {
		t.Errorf("installed skill = %q", data)
	}
}

func TestSkillInstallRefusesToClobber(t *testing.T) {
	SetSkill("skill")
	t.Cleanup(func() { SetSkill("") })
	dir := filepath.Join(t.TempDir(), "skills")

	run(t, "skill", "install", "--dir", dir)
	if _, err := tryRun("skill", "install", "--dir", dir); err == nil {
		t.Error("expected a refusal to overwrite without --force")
	}
	if _, err := tryRun("skill", "install", "--dir", dir, "--force"); err != nil {
		t.Errorf("--force should overwrite: %v", err)
	}
}

func TestSkillPrintGoesToStdout(t *testing.T) {
	SetSkill("---\nname: svg2drawio\n---\n")
	t.Cleanup(func() { SetSkill("") })

	out := run(t, "skill", "install", "--print")
	if !strings.Contains(out, "name: svg2drawio") {
		t.Errorf("printed skill = %q", out)
	}
}

func TestSkillPathPointsAtClaudeSkills(t *testing.T) {
	out := run(t, "skill", "path")
	if !strings.Contains(out, filepath.Join(".claude", "skills", "svg2drawio", "SKILL.md")) {
		t.Errorf("skill path = %q", out)
	}
}

func TestCompletionIsAvailable(t *testing.T) {
	out := run(t, "completion", "zsh")
	if !strings.Contains(out, "compdef") {
		t.Errorf("zsh completion looks wrong: %.120q", out)
	}
}
