package starters

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ems-bridge/expr"
	"ems-bridge/messages"
)

// EventType represents the kind of file-system event a FileEventStarter reacts to.
type EventType string

const (
	EventTypeFileCreate EventType = "FILE_CREATE"
	EventTypeFileChange EventType = "FILE_CHANGE"
	EventTypeFileDelete EventType = "FILE_DELETE"
)

func parseEventType(s string) (EventType, error) {
	switch EventType(s) {
	case EventTypeFileCreate, EventTypeFileChange, EventTypeFileDelete:
		return EventType(s), nil
	default:
		return "", fmt.Errorf("unknown eventType %q: must be FILE_CREATE, FILE_CHANGE or FILE_DELETE", s)
	}
}

// FileEventStarter watches an input folder for file-system events and
// forwards matching files downstream.
type FileEventStarter struct {
	StarterConfig
	inputFolder              string
	eventType                EventType
	processExistingOnStartup bool
	checkSubfolders          bool
	filenameSuffixes         []string
	filenamePrefixes         []string
	filenameRegexes          []string
	outputFolder             string
	handler                  Handler
}

func newFileEventStarter(s StarterConfig, handler Handler) (*FileEventStarter, error) {
	p := s.Properties

	// Check required properties
	for _, prop := range []string{"inputFolder", "eventType"} {
		if _, exists := p[prop]; !exists {
			return nil, fmt.Errorf("starter %q: missing required property %q", s.ID, prop)
		}
	}

	eventType, err := parseEventType(p["eventType"])
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid eventType: %w", s.ID, err)
	}

	processExisting, err := expr.Bool(p["processExistingOnStartup"])
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid processExistingOnStartup: %w", s.ID, err)
	}

	checkSubs, err := expr.Bool(p["checkSubfolders"])
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid checkSubfolders: %w", s.ID, err)
	}

	suffixesExpr, exists := p["filenameSuffixes"]
	if !exists {
		suffixesExpr = "[]"
	}
	suffixes, err := expr.StringSlice(suffixesExpr)
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid filenameSuffixes: %w", s.ID, err)
	}

	prefixesExpr, exists := p["filenamePrefixes"]
	if !exists {
		prefixesExpr = "[]"
	}
	prefixes, err := expr.StringSlice(prefixesExpr)
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid filenamePrefixes: %w", s.ID, err)
	}

	regexesExpr, exists := p["filenameRegexes"]
	if !exists {
		regexesExpr = "[]"
	}
	regexes, err := expr.StringSlice(regexesExpr)
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid filenameRegexes: %w", s.ID, err)
	}

	return &FileEventStarter{
		StarterConfig:            s,
		inputFolder:              p["inputFolder"],
		eventType:                eventType,
		processExistingOnStartup: processExisting,
		checkSubfolders:          checkSubs,
		filenameSuffixes:         suffixes,
		filenamePrefixes:         prefixes,
		filenameRegexes:          regexes,
		outputFolder:             p["outputFolder"],
		handler:                  handler,
	}, nil
}

func (s *FileEventStarter) Start() error {
	slog.Info("FileEventStarter starting", "id", s.ID, "inputFolder", s.inputFolder, "eventType", s.eventType)

	if s.processExistingOnStartup {
		return s.scanExisting()
	}
	return nil
}

func (s *FileEventStarter) scanExisting() error {
	walk := filepath.Walk
	if !s.checkSubfolders {
		walk = func(root string, fn filepath.WalkFunc) error {
			entries, err := os.ReadDir(root)
			if err != nil {
				return err
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil {
					return err
				}
				if err := fn(filepath.Join(root, e.Name()), info, nil); err != nil {
					return err
				}
			}
			return nil
		}
	}

	return walk(s.inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("FileEventStarter: error accessing path", "id", s.ID, "path", path, "err", err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if s.matches(info.Name()) {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading file %q: %w", path, err)
			}
			msg := messages.NewMessage(
				data,
				map[string]string{
					"filename":    info.Name(),
					"path":        path,
					"inputFolder": s.inputFolder,
				},
				map[string]any{},
			)
			slog.Info("FileEventStarter: message created", "id", s.ID, "message", msg.String())
			msg.Print()
			if err := s.handler(msg); err != nil {
				return fmt.Errorf("handler error for %q: %w", path, err)
			}
			if s.outputFolder != "" {
				if err := s.moveFile(path, info.Name()); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *FileEventStarter) matches(name string) bool {
	if len(s.filenameSuffixes) > 0 {
		matched := false
		for _, suffix := range s.filenameSuffixes {
			if strings.HasSuffix(name, suffix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(s.filenamePrefixes) > 0 {
		matched := false
		for _, prefix := range s.filenamePrefixes {
			if strings.HasPrefix(name, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, pattern := range s.filenameRegexes {
		re, err := regexp.Compile(pattern)
		if err != nil {
			slog.Warn("FileEventStarter: invalid regex", "id", s.ID, "pattern", pattern, "err", err)
			continue
		}
		if !re.MatchString(name) {
			return false
		}
	}

	return true
}

func (s *FileEventStarter) moveFile(src, name string) error {
	if err := os.MkdirAll(s.outputFolder, 0755); err != nil {
		return fmt.Errorf("FileEventStarter %q: creating output folder %q: %w", s.ID, s.outputFolder, err)
	}
	dst := filepath.Join(s.outputFolder, name)
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("FileEventStarter %q: moving %q to %q: %w", s.ID, src, dst, err)
	}
	slog.Info("FileEventStarter: file moved to output folder", "id", s.ID, "src", src, "dst", dst)
	return nil
}

func (s *FileEventStarter) Stop() error {
	slog.Info("FileEventStarter stopped", "id", s.ID)
	return nil
}
