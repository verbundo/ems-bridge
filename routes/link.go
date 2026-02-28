package routes

// Link is a directed edge connecting two nodes within a route.
type Link struct {
	From string
	To   string
}

func newLink(cfg LinkConfig) Link {
	return Link{
		From: cfg.From,
		To:   cfg.To,
	}
}
