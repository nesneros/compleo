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
	carapacespec.AddMacro("none", carapacespec.MacroN(func() carapace.Action { return carapace.ActionValues() }),
		"No completion candidates")
}

// actionScripts implements $compleo.scripts: like $executables, narrowed to compleo-wrapped
// executables, merged with plain file completion.
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

// IsCompleoScript reports whether path looks like a compleo-wrapped script: a shebang plus
// "compleo" somewhere in its first 4KB. Cheap, not a guarantee.
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
