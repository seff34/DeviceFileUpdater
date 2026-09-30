// Package shell holds helpers for talking to POSIX shells on devices.
package shell

import "strings"

// Quote wraps s in single quotes so any POSIX sh treats it as one literal word.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
