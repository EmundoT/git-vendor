package git

import "context"

// UserIdentity returns the configured user in "Name <email>" format.
// Returns empty string if not configured.
func (g *Git) UserIdentity(ctx context.Context) string {
	name, _ := g.Run(ctx, "config", "user.name")
	email, _ := g.Run(ctx, "config", "user.email")
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	case email != "":
		return email
	default:
		return ""
	}
}

// ConfigGet reads a git config value.
func (g *Git) ConfigGet(ctx context.Context, key string) (string, error) {
	return g.Run(ctx, "config", key)
}

// ConfigSet writes a git config value.
func (g *Git) ConfigSet(ctx context.Context, key, value string) error {
	return g.RunSilent(ctx, "config", key, value)
}
