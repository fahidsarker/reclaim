// Package exec permanently deletes or trashes planned targets and journals runs.
//
// Run re-validates each VerdictDelete target (existence, inode identity, root
// containment, not a symlink), removes deepest-first, continues on failure,
// and appends a JSONL journal record for every mutating run.
package exec
