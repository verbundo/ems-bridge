package components

import (
	"fmt"
	"log/slog"
)

// JmsComponent represents a JMS broker connection.
type JmsComponent struct {
	Component
	provider string
	url      string
	username string
	password string
}

func newJmsComponent(cfg Component) (*JmsComponent, error) {
	p := cfg.Properties
	return &JmsComponent{
		Component: cfg,
		provider:  p["provider"],
		url:       p["url"],
		username:  p["username"],
		password:  p["password"],
	}, nil
}

func (c *JmsComponent) Start() error {
	msg := fmt.Sprintf("JmsComponent %s connecting to url %s as user %s", c.Name, c.url, c.username)
	slog.Info(msg)
	return nil
}

func (c *JmsComponent) Stop() error {
	slog.Info("JmsComponent disconnected", "name", c.Name)
	return nil
}
