//go:build linux

// Package netscope answers one question about the host it runs on: which
// networks can this machine actually reach, and how do we know?
//
// It is read-only — no route installation, no interface changes, no state
// changes of any kind — and Linux-only, using netlink instead of shelling
// out to external tools. Callers select which cost/risk tier of source to
// run (see Level) and get back a Report that distinguishes "found nothing"
// from "could not look" for every source it tried.
package netscope
