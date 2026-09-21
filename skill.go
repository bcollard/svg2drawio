// Package svg2drawio exposes the repo-root assets compiled into the binary.
package svg2drawio

import _ "embed"

// SkillMD is the canonical svg2drawio Agent Skill definition, embedded from the
// repo-root SKILL.md. `svg2drawio skill install` ships it inside the binary, so
// the installed CLI can drop the skill into an AI coding tool's skills
// directory without a separate download. SKILL.md stays the single source of
// truth.
//
//go:embed SKILL.md
var SkillMD string
