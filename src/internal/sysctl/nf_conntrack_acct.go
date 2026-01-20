package sysctl

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"conntrack-exporter/internal/procfs"
)

// nfConntrackAcctRelPath is the procfs-relative path to the sysctl value.
//
// sysctl net.netfilter.nf_conntrack_acct is exposed as:
//   /proc/sys/net/netfilter/nf_conntrack_acct
//
// When set to 1, the kernel records packets/bytes counters in `/proc/net/nf_conntrack`.
const nfConntrackAcctRelPath = "sys/net/netfilter/nf_conntrack_acct"

// ReadNfConntrackAcct returns the current value of net.netfilter.nf_conntrack_acct.
// If the file cannot be read, an error is returned.
func ReadNfConntrackAcct(fs procfs.FS) (int, error) {
	b, err := fs.ReadFile(nfConntrackAcctRelPath)
	if err != nil {
		return 0, err
	}

	s := strings.TrimSpace(string(bytes.TrimSpace(b)))
	if s == "" {
		return 0, fmt.Errorf("%s is empty", fs.Path(nfConntrackAcctRelPath))
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", fs.Path(nfConntrackAcctRelPath), s, err)
	}

	return v, nil
}

// ConfigureNfConntrackAcct attempts to set net.netfilter.nf_conntrack_acct=1.
//
// This typically requires root privileges. If it fails, the caller should log
// a warning and continue running (exporter can still work, but bytes/packets
// may be missing from conntrack entries).
func ConfigureNfConntrackAcct(fs procfs.FS) error {
	// The sysctl proc file expects a newline-terminated value.
	if err := fs.WriteFile(nfConntrackAcctRelPath, []byte("1\n"), 0o644); err != nil {
		return err
	}

	// Verify after writing (best-effort).
	v, err := ReadNfConntrackAcct(fs)
	if err != nil {
		return err
	}
	if v != 1 {
		return fmt.Errorf("failed to set %s to 1 (current=%d)", fs.Path(nfConntrackAcctRelPath), v)
	}

	return nil
}

