package spec

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"

	"github.com/carapace-sh/carapace"
	carapacespec "github.com/carapace-sh/carapace-spec"
)

func init() {
	carapacespec.AddMacro("scripts", carapacespec.MacroN(func() carapace.Action { return actionScripts() }),
		`Executables on $PATH that look like they wrap compleo (have a shebang and mention "compleo")`)
}

// actionScripts implements $compleo.scripts: like $executables, narrowed to compleo-wrapped
// ones, merged with plain file completion (generate also accepts a path, unfiltered there —
// carapace's path completion has no hook to filter by file content).
func actionScripts() carapace.Action {
	return carapace.Batch(actionPathScripts(), carapace.ActionFiles()).ToA()
}

func actionPathScripts() carapace.Action {
	return carapace.ActionCallback(func(c carapace.Context) carapace.Action {
		seen := make(map[string]bool)
		var names []string
		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if seen[name] || e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil || info.Mode()&0o111 == 0 {
					continue
				}
				if IsCompleoScript(filepath.Join(dir, name)) {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
		sort.Strings(names)
		return carapace.ActionValues(names...).Tag("compleo scripts")
	})
}

// IsCompleoScript checks for a shebang plus "compleo" in path's first 4KB — cheap, not a
// guarantee. Used by $compleo.scripts and generate's sanity check.
func IsCompleoScript(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck

	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	head := buf[:n]
	return len(head) > 2 && head[0] == '#' && head[1] == '!' && bytes.Contains(head, []byte("compleo"))
}
