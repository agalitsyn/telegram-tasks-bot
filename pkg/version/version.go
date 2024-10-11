package version

import (
	"fmt"
	"runtime/debug"
	"time"
)

var (
	Tag      string
	Revision string
	BuildAt  string
	Dirty    bool
)

func init() {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, setting := range buildInfo.Settings {
		// https://pkg.go.dev/runtime/debug#BuildSetting
		switch setting.Key {
		case "vcs.revision":
			Revision = setting.Value
		case "vcs.time":
			BuildAt = setting.Value
		case "vcs.modified":
			if setting.Value == "true" {
				Dirty = true
			}
		}
	}
}

func String() string {
	// go run
	if Revision == "" {
		return "dev"
	} else {
		Revision = Revision[:7]
	}

	t, err := time.Parse(time.RFC3339, BuildAt)
	if err == nil {
		BuildAt = t.Format("2006-01-02 15:04:05")
	}

	s := fmt.Sprintf("%s %s at %s", Tag, Revision, BuildAt)
	if Dirty {
		s += " dirty"
	}
	return s
}
