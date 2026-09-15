package conntrackhistory

import (
	"fmt"
	"io"
	"strings"
)

// WriteText renders conntrack events as kernel-state context.
func WriteText(writer io.Writer, result QueryResult) error {
	if len(result.Events) == 0 {
		_, err := fmt.Fprintln(
			writer,
			"No qualifying conntrack events found.",
		)
		return err
	}

	for index, current := range result.Events {
		if index > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}

		status := statusNames(current.Status)
		statusText := "none"
		if len(status) > 0 {
			statusText = strings.Join(status, ",")
		}

		if _, err := fmt.Fprintf(
			writer,
			"%s  %s  %s",
			current.ObservedAt.UTC().Format(
				"2006-01-02 15:04:05.000000 UTC",
			),
			current.EventType,
			current.Original.Protocol,
		); err != nil {
			return err
		}
		if current.TCPState != "" &&
			current.TCPState != "not_known" {
			if _, err := fmt.Fprintf(
				writer,
				"  state=%s",
				current.TCPState,
			); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(
			writer,
			"  original: %s\n",
			current.Original.String(),
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"  reply:    %s\n",
			current.Reply.String(),
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"  status:   %s\n",
			statusText,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"  mark:     %d\n",
			current.Mark,
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			writer,
			"  source:   %s\n",
			current.TimestampSource,
		); err != nil {
			return err
		}
	}

	return nil
}
