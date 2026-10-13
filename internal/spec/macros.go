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
	carapacespec.AddMacro("scripts", carapacespec.MacroN(func() carapace.Action { return ActionScripts() }),
		`Executables on $PATH that look like they wrap compleo (have a shebang and mention "compleo")`)
}

// ActionScripts implements $compleo.scripts: like $executables, but narrowed to executables
// that look like they wrap compleo, for "compleo generate <TAB>" (any PATH binary is useless there).
func ActionScripts() carapace.Action {
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
				if looksWrapped(filepath.Join(dir, name)) {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
		sort.Strings(names)
		return carapace.ActionValues(names...).Tag("compleo scripts")
	})
}

// looksWrapped checks path's first 4KB for a shebang (is this a script) plus "compleo"
// (does it call compleo) — done natively instead of shelling out to "file" per candidate.
func looksWrapped(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }() // read-only; a close error here doesn't change the answer

	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	head := buf[:n]
	return len(head) > 2 && head[0] == '#' && head[1] == '!' && bytes.Contains(head, []byte("compleo"))
}
