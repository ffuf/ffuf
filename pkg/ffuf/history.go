package ffuf

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ConfigOptionsHistory struct {
	ConfigOptions
	Time time.Time `json:"time"`
}

func WriteHistoryEntry(conf *Config) (string, error) {
	if conf.Options == nil {
		return "", errors.New("cannot write history entry: config has no source options")
	}
	options := ConfigOptionsHistory{
		ConfigOptions: historyOptions(conf),
		Time:          time.Now(),
	}
	jsonoptions, err := json.Marshal(options)
	if err != nil {
		return "", err
	}
	hashstr := calculateHistoryHash(jsonoptions)
	err = createConfigDir(filepath.Join(HISTORYDIR, hashstr))
	if err != nil {
		return "", err
	}
	err = os.WriteFile(filepath.Join(HISTORYDIR, hashstr, "options"), jsonoptions, 0640)
	return hashstr, err
}

// historyOptions projects the Config's retained source options into the form
// persisted for FFUFHASH history. Most fields ride the retained snapshot
// unchanged, but fields the engine mutates AFTER parsing must be refreshed from
// the live Config — otherwise a `-search` reconstruction shows stale values:
//   - HTTP.URL: recursion rewrites conf.Url per queued job (job.go prepareQueueJob),
//     so the frozen snapshot would report the base URL, not the recursed path.
//   - Filter/Matcher.*: autocalibration installs filters at runtime, absent from
//     the raw input. Rebuilt from MatcherManager exactly as the old ToOptions did.
func historyOptions(conf *Config) ConfigOptions {
	o := *conf.Options
	o.HTTP.URL = conf.Url
	if conf.MatcherManager != nil {
		o.Filter.Mode = conf.FilterMode
		o.Filter.Lines, o.Filter.Regexp, o.Filter.Size = "", "", ""
		o.Filter.Status, o.Filter.Time, o.Filter.Words = "", "", ""
		for name, f := range conf.MatcherManager.GetFilters() {
			switch name {
			case "line":
				o.Filter.Lines = f.Repr()
			case "regexp":
				o.Filter.Regexp = f.Repr()
			case "size":
				o.Filter.Size = f.Repr()
			case "status":
				o.Filter.Status = f.Repr()
			case "time":
				o.Filter.Time = f.Repr()
			case "words":
				o.Filter.Words = f.Repr()
			}
		}
		o.Matcher.Mode = conf.MatcherMode
		o.Matcher.Lines, o.Matcher.Regexp, o.Matcher.Size = "", "", ""
		o.Matcher.Status, o.Matcher.Time, o.Matcher.Words = "", "", ""
		for name, f := range conf.MatcherManager.GetMatchers() {
			switch name {
			case "line":
				o.Matcher.Lines = f.Repr()
			case "regexp":
				o.Matcher.Regexp = f.Repr()
			case "size":
				o.Matcher.Size = f.Repr()
			case "status":
				o.Matcher.Status = f.Repr()
			case "time":
				o.Matcher.Time = f.Repr()
			case "words":
				o.Matcher.Words = f.Repr()
			}
		}
	}
	return o
}

func calculateHistoryHash(options []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(options))
}

func SearchHash(hash string) ([]ConfigOptionsHistory, int, error) {
	coptions := make([]ConfigOptionsHistory, 0)
	if len(hash) < 6 {
		return coptions, 0, errors.New("bad FFUFHASH value")
	}
	historypart := hash[0:5]
	position, err := strconv.ParseInt(hash[5:], 16, 32)
	if err != nil {
		return coptions, 0, errors.New("bad positional value in FFUFHASH")
	}
	all_dirs, err := os.ReadDir(HISTORYDIR)
	if err != nil {
		return coptions, 0, err
	}
	matched_dirs := make([]string, 0)
	for _, filename := range all_dirs {
		if filename.IsDir() {
			if strings.HasPrefix(strings.ToLower(filename.Name()), strings.ToLower(historypart)) {
				matched_dirs = append(matched_dirs, filename.Name())
			}
		}
	}
	for _, dirname := range matched_dirs {
		copts, err := configFromHistory(filepath.Join(HISTORYDIR, dirname))
		if err != nil {
			continue
		}
		coptions = append(coptions, copts)

	}
	return coptions, int(position), err
}

func HistoryReplayable(conf *Config) (bool, string) {
	for _, w := range conf.Wordlists {
		if w == "-" || strings.HasPrefix(w, "-:") {
			return false, "stdin input was used for one of the wordlists"
		}
	}
	return true, ""
}

func configFromHistory(dirname string) (ConfigOptionsHistory, error) {
	jsonOptions, err := os.ReadFile(filepath.Join(dirname, "options"))
	if err != nil {
		return ConfigOptionsHistory{}, err
	}
	tmpOptions := ConfigOptionsHistory{}
	err = json.Unmarshal(jsonOptions, &tmpOptions)
	return tmpOptions, err
}
