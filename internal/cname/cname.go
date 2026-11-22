// Package cname turns C's names for enum values into Go strings.
package cname

import (
	"fmt"
	"strings"
)

// Value turns C's name for an enum value, such as "TOX_CONNECTION_TCP" for
// prefix "TOX_CONNECTION_", into "tcp". It returns false for a name without
// the prefix, which is what C gives for a value it doesn't know.
func Value(name, prefix string) (string, bool) {
	rest, ok := strings.CutPrefix(name, prefix)
	return words(rest), ok
}

// words turns "FRIEND_ADD" into "friend add".
func words(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", " ")
}

// String is the String of an enum type named typeName.
func String(name, prefix, typeName string, value uint32) string {
	if s, ok := Value(name, prefix); ok {
		return s
	}
	return fmt.Sprintf("%s(%d)", typeName, value)
}

// Error turns C's name for an error, such as "TOX_ERR_FRIEND_ADD_TOO_LONG" for
// prefix "TOX_ERR_FRIEND_ADD_", into "pkg: friend add: too long".
func Error(pkg, name, prefix string, code uint32) string {
	op := words(strings.TrimSuffix(strings.TrimPrefix(prefix, "TOX_ERR_"), "_"))
	if s, ok := Value(name, prefix); ok {
		return pkg + ": " + op + ": " + s
	}
	return fmt.Sprintf("%s: %s: error %d", pkg, op, code)
}
