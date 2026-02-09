package git

import (
	"context"
	"strings"
)

// NoteRef is a reference namespace for git notes (e.g., "refs/notes/agent").
type NoteRef string

// NoteEntry represents an entry in a git notes listing.
type NoteEntry struct {
	NoteHash   string // SHA of the note blob
	CommitHash string // SHA of the annotated commit
}

// AddNote adds or overwrites a note on commitHash under the given NoteRef namespace.
func (g *Git) AddNote(ctx context.Context, ref NoteRef, commitHash, content string) error {
	return g.RunSilent(ctx, "notes", "--ref="+string(ref), "add", "-f", "-m", content, commitHash)
}

// GetNote returns the note content attached to commitHash under the given NoteRef namespace.
// GetNote returns a *GitError if no note exists for the commit.
func (g *Git) GetNote(ctx context.Context, ref NoteRef, commitHash string) (string, error) {
	return g.Run(ctx, "notes", "--ref="+string(ref), "show", commitHash)
}

// RemoveNote removes the note on commitHash under the given NoteRef namespace.
func (g *Git) RemoveNote(ctx context.Context, ref NoteRef, commitHash string) error {
	return g.RunSilent(ctx, "notes", "--ref="+string(ref), "remove", commitHash)
}

// ListNotes returns all note entries under the given NoteRef namespace.
// Each entry maps a note blob SHA to the annotated commit SHA.
// ListNotes returns an empty slice when no notes exist.
func (g *Git) ListNotes(ctx context.Context, ref NoteRef) ([]NoteEntry, error) {
	lines, err := g.RunLines(ctx, "notes", "--ref="+string(ref), "list")
	if err != nil {
		return nil, err
	}
	if lines == nil {
		return nil, nil
	}
	entries := make([]NoteEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		entries = append(entries, NoteEntry{
			NoteHash:   parts[0],
			CommitHash: parts[1],
		})
	}
	return entries, nil
}
