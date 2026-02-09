package main

import (
	"fmt"
	"regexp"
)

var ticketFooterPattern = regexp.MustCompile(`^GAIT run_id=([A-Za-z0-9_-]+) manifest=sha256:([a-f0-9]{64}) verify="gait verify ([A-Za-z0-9_-]+)"$`)

func formatTicketFooter(runID string, manifestDigest string) string {
	return fmt.Sprintf(
		`GAIT run_id=%s manifest=sha256:%s verify="gait verify %s"`,
		runID,
		manifestDigest,
		runID,
	)
}

func ticketFooterMatchesContract(value string) bool {
	matches := ticketFooterPattern.FindStringSubmatch(value)
	if len(matches) != 4 {
		return false
	}
	return matches[1] == matches[3]
}
