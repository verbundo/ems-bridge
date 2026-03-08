package starters

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

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
	lockFileSuffix           string
	pollingInterval          time.Duration
	sem                      chan struct{} // limits concurrent file processing
	done                     chan struct{}
	stopOnce                 sync.Once
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

	lockFileSuffix := p["lockFileSuffix"]
	if lockFileSuffix == "" {
		lockFileSuffix = ".lock"
	}

	intervalStr := p["pollingInterval"]
	if intervalStr == "" {
		intervalStr = "2s"
	}
	pollingInterval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid pollingInterval: %w", s.ID, err)
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
		lockFileSuffix:           lockFileSuffix,
		pollingInterval:          pollingInterval,
		sem:                      make(chan struct{}, 4),
		done:                     make(chan struct{}),
		handler:                  handler,
	}, nil
}

func (s *FileEventStarter) Start() error {
	slog.Info("FileEventStarter starting", "id", s.ID, "inputFolder", s.inputFolder, "eventType", s.eventType, "pollingInterval", s.pollingInterval)
	go s.poll()
	return nil
}

func (s *FileEventStarter) poll() {
	if !s.processExistingOnStartup {
		// wait for the first interval before scanning
		select {
		case <-s.done:
			return
		case <-time.After(s.pollingInterval):
		}
	}
	for {
		if err := s.scanExisting(); err != nil {
			slog.Error("FileEventStarter: scan error", "id", s.ID, "err", err)
		}
		select {
		case <-s.done:
			return
		case <-time.After(s.pollingInterval):
		}
	}
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

	var wg sync.WaitGroup
	walkErr := walk(s.inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("FileEventStarter: error accessing path", "id", s.ID, "path", path, "err", err)
			return nil
		}
		if info.IsDir() || strings.HasSuffix(info.Name(), s.lockFileSuffix) {
			return nil
		}
		if s.matches(info.Name()) {
			s.sem <- struct{}{} // block until a slot is free
			wg.Add(1)
			go func(path string, info os.FileInfo) {
				defer wg.Done()
				defer func() { <-s.sem }()
				if err := s.processFile(path, info); err != nil {
					slog.Error("FileEventStarter: error processing file", "id", s.ID, "path", path, "err", err)
				}
			}(path, info)
		}
		return nil
	})
	wg.Wait()
	s.cleanStaleLocks()
	return walkErr
}

// cleanStaleLocks removes any lock files left in the input folder after all
// processing goroutines have finished. Remaining locks are stale (e.g. from a
// previous crashed run) and safe to delete at this point.
func (s *FileEventStarter) cleanStaleLocks() {
	entries, err := os.ReadDir(s.inputFolder)
	if err != nil {
		slog.Warn("FileEventStarter: cannot read input folder for lock cleanup", "id", s.ID, "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), s.lockFileSuffix) {
			continue
		}
		lockPath := filepath.Join(s.inputFolder, e.Name())
		if err := os.Remove(lockPath); err != nil {
			slog.Warn("FileEventStarter: failed to remove stale lock file", "id", s.ID, "lockFile", lockPath, "err", err)
		} else {
			slog.Warn("FileEventStarter: removed stale lock file", "id", s.ID, "lockFile", lockPath)
		}
	}
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

func (s *FileEventStarter) processFile(path string, info os.FileInfo) error {
	acquired, err := s.acquireLock(path)
	if err != nil {
		return err
	}
	if !acquired {
		slog.Info("FileEventStarter: file locked by another instance, skipping", "id", s.ID, "path", path)
		return nil
	}
	defer s.releaseLock(path)

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
	slog.Info("FileEventStarter: message created", "id", s.ID, "message", msg)
	msg.Print()

	if err := s.handler(msg); err != nil {
		return fmt.Errorf("handler error for %q: %w", path, err)
	}

	if s.outputFolder != "" {
		if err := s.moveFile(path, info.Name()); err != nil {
			return err
		}
	}
	return nil
}

// acquireLock creates a .lock file atomically. Returns false (without error) if
// the lock file already exists, meaning another instance holds the lock.
func (s *FileEventStarter) acquireLock(path string) (bool, error) {
	lockPath := path + s.lockFileSuffix
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("creating lock file %q: %w", lockPath, err)
	}
	f.Close()
	slog.Info("FileEventStarter: lock acquired", "id", s.ID, "lockFile", lockPath)
	return true, nil
}

func (s *FileEventStarter) releaseLock(path string) {
	lockPath := path + s.lockFileSuffix
	if err := os.Remove(lockPath); err != nil {
		slog.Warn("FileEventStarter: failed to remove lock file", "id", s.ID, "lockFile", lockPath, "err", err)
		return
	}
	slog.Info("FileEventStarter: lock released", "id", s.ID, "lockFile", lockPath)
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
	s.stopOnce.Do(func() { close(s.done) })
	slog.Info("FileEventStarter stopped", "id", s.ID)
	return nil
}
