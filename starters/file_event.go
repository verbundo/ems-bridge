package starters

import (
	"fmt"
	"strconv"
)

// FileEventStarter watches an input folder for file-system events and
// forwards matching files downstream.
type FileEventStarter struct {
	StarterConfig
	inputFolder             string
	eventType               string
	processExistingOnStartup bool
	checkSubfolders         bool
	filenameSuffixes        string
	filenamePrefixes        string
	outputFolder            string
}

func newFileEventStarter(s StarterConfig) (*FileEventStarter, error) {
	p := s.Properties

	processExisting, err := strconv.ParseBool(p["processExistingOnStartup"])
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid processExistingOnStartup: %w", s.ID, err)
	}

	checkSubs, err := strconv.ParseBool(p["checkSubfolders"])
	if err != nil {
		return nil, fmt.Errorf("starter %q: invalid checkSubfolders: %w", s.ID, err)
	}

	return &FileEventStarter{
		StarterConfig:           s,
		inputFolder:             p["inputFolder"],
		eventType:               p["eventType"],
		processExistingOnStartup: processExisting,
		checkSubfolders:         checkSubs,
		filenameSuffixes:        p["filenameSuffixes"],
		filenamePrefixes:        p["filenamePrefixes"],
		outputFolder:            p["outputFolder"],
	}, nil
}

func (s *FileEventStarter) Start() error {
	fmt.Printf("FileEventStarter %q: watching %q for %s events\n", s.ID, s.inputFolder, s.eventType)
	return nil
}

func (s *FileEventStarter) Stop() error {
	fmt.Printf("FileEventStarter %q: stopped\n", s.ID)
	return nil
}
