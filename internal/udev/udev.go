// Package udev carries the udev rule that grants the desktop user access to
// Logitech hidraw nodes.
package udev

import _ "embed"

// RuleName is the file name the rule should be installed under.
const RuleName = "70-logirgb.rules"

// Rule is the content of the udev rule.
//
//go:embed 70-logirgb.rules
var Rule string
