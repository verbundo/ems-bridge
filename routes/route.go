package routes

import (
	"fmt"
	"log/slog"

	"ems-bridge/messages"
	"ems-bridge/processors"
	"ems-bridge/starters"
)

// RouteConfig holds the parsed configuration for a single route.
type RouteConfig struct {
	Name             string                       `yaml:"name"`
	StarterConfigs   []starters.StarterConfig     `yaml:"starters"`
	ProcessorConfigs []processors.ProcessorConfig `yaml:"processors"`
	LinkConfigs      []LinkConfig                 `yaml:"links"`
}

// LinkConfig describes a directed edge between two nodes in a route.
type LinkConfig struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// Route is a fully instantiated route with live starters, processors and links.
type Route struct {
	Name           string
	Starters       []starters.Runner
	Processors     []processors.Runner
	Links          []Link
	FirstProcessor processors.Runner
	processorIDs   []string // parallel to Processors, used by Init
}

// New creates a Route from cfg, instantiating all starters, processors and links.
func New(cfg RouteConfig) (*Route, error) {
	route := &Route{Name: cfg.Name}

	for _, sc := range cfg.StarterConfigs {
		s, err := starters.New(sc, route.Execute)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", cfg.Name, err)
		}
		route.Starters = append(route.Starters, s)
	}

	for _, pc := range cfg.ProcessorConfigs {
		p, err := processors.New(pc)
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", cfg.Name, err)
		}
		route.Processors = append(route.Processors, p)
		route.processorIDs = append(route.processorIDs, pc.ID)
	}

	for _, lc := range cfg.LinkConfigs {
		route.Links = append(route.Links, newLink(lc))
	}

	return route, nil
}

// Init resolves the first processor — the one not referenced as a "to" in any
// link. Returns an error if more than one such processor is found.
func (r *Route) Init() error {
	slog.Info("initializing route", "name", r.Name)
	toIDs := make(map[string]bool, len(r.Links))
	for _, l := range r.Links {
		toIDs[l.To] = true
	}

	var first processors.Runner
	for i, id := range r.processorIDs {
		if !toIDs[id] {
			if first != nil {
				return fmt.Errorf("route %q: multiple first processors found", r.Name)
			}
			first = r.Processors[i]
		}
	}

	r.FirstProcessor = first
	for _, s := range r.Starters {
		if err := s.Start(); err != nil {
			return fmt.Errorf("route %q: starting starter: %w", r.Name, err)
		}
	}
	return nil
}

// Execute runs msg through the processor chain starting from FirstProcessor,
// following Links in order.
func (r *Route) Execute(msg *messages.Message) error {
	// Build ID -> Runner and ID -> nextID maps.
	byID := make(map[string]processors.Runner, len(r.Processors))
	for i, id := range r.processorIDs {
		byID[id] = r.Processors[i]
	}
	nextID := make(map[string]string, len(r.Links))
	for _, l := range r.Links {
		nextID[l.From] = l.To
	}

	// Find the ID of FirstProcessor.
	var currentID string
	for i, p := range r.Processors {
		if p == r.FirstProcessor {
			currentID = r.processorIDs[i]
			break
		}
	}

	slog.Info("route input message", "route", r.Name, "message", msg)

	for currentID != "" {
		p, ok := byID[currentID]
		if !ok {
			return fmt.Errorf("route %q: processor %q not found", r.Name, currentID)
		}
		slog.Info("executing processor", "route", r.Name, "processor", currentID)
		if err := p.Process(msg); err != nil {
			return fmt.Errorf("route %q: processor %q: %w", r.Name, currentID, err)
		}
		currentID = nextID[currentID]
	}

	slog.Info("route output message", "route", r.Name, "message", msg)
	return nil
}
