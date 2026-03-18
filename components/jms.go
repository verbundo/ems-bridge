package components

import (
	"fmt"
	"log/slog"

	"ems-bridge/components/jms/tibco"
)

// JmsComponent represents a JMS broker connection.
type JmsComponent struct {
	Component
	provider string
	url      string
	username string
	password string
	conn     *tibco.Connection
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

// Conn returns the live TIBCO EMS connection, or nil if the component has not
// been started yet or does not use the tibems provider.
func (c *JmsComponent) Conn() *tibco.Connection { return c.conn }

func (c *JmsComponent) Start() error {
	if c.provider != "tibems" {
		return nil
	}
	slog.Info("JmsComponent connecting", "name", c.Name, "url", c.url, "user", c.username)
	conn, err := tibco.EMS_NewConnection(c.url, c.username, c.password)
	if err != nil {
		return fmt.Errorf("JmsComponent %q: %w", c.Name, err)
	}
	c.conn = conn
	slog.Info("JmsComponent connected", "name", c.Name)
	return nil
}

func (c *JmsComponent) Stop() error {
	if c.conn == nil {
		return nil
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("JmsComponent %q: %w", c.Name, err)
	}
	c.conn = nil
	slog.Info("JmsComponent disconnected", "name", c.Name)
	return nil
}
