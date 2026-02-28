package routes

import (
	"fmt"

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
		s, err := starters.New(sc)
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
	return nil
}
