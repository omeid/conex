package conex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/client"
	"github.com/moby/term"
)

var (
	progressOut  io.Writer = os.Stderr
	progressFd             = os.Stderr.Fd()
	isTerminalFn           = term.IsTerminal
)

// printPullProgress decodes the JSON messages from the ImagePullResponse and
// prints progress updates to os.Stderr. It renders an in-place progress-bar
// on interactive terminals, and logs discrete status changes on non-terminals/CIs.
func printPullProgress(ctx context.Context, response client.ImagePullResponse) error {
	defer func() { _ = response.Close() }()

	isTerminal := isTerminalFn(progressFd)
	layers := make(map[string]int) // id -> line index
	var layerList []string
	lastStatus := make(map[string]string)
	var extraLines int

	for msg, err := range response.JSONMessages(ctx) {
		if err != nil {
			return err
		}
		if msg.Error != nil {
			return msg.Error
		}

		if isTerminal {
			if msg.ID != "" {
				idx, ok := layers[msg.ID]
				if !ok {
					idx = len(layerList)
					layers[msg.ID] = idx
					layerList = append(layerList, msg.ID)
					// Print a new line to allocate space for this layer
					_, _ = fmt.Fprintln(progressOut)
				}
				// Go up to the layer's line, print, and go back down
				diff := len(layerList) - idx + extraLines
				barStr := formatProgress(msg.Progress)
				_, _ = fmt.Fprintf(progressOut, "\033[%dA\r\033[K%-12s: %-15s%s\033[%dB\r", diff, msg.ID, msg.Status, barStr, diff)
			} else if msg.Status != "" {
				_, _ = fmt.Fprintln(progressOut, msg.Status)
				extraLines++
			}
		} else {
			if msg.ID != "" {
				if lastStatus[msg.ID] != msg.Status {
					lastStatus[msg.ID] = msg.Status
					_, _ = fmt.Fprintf(progressOut, "%-12s: %s\n", msg.ID, msg.Status)
				}
			} else if msg.Status != "" {
				_, _ = fmt.Fprintln(progressOut, msg.Status)
			}
		}
	}
	return nil
}

func formatProgress(p *jsonstream.Progress) string {
	if p == nil || p.Total <= 0 {
		return ""
	}
	const barWidth = 20
	current := p.Current
	total := p.Total
	if current > total {
		current = total
	}
	percent := float64(current) / float64(total)
	filled := int(percent * barWidth)

	bar := make([]byte, barWidth)
	for i := range barWidth {
		if i < filled {
			bar[i] = '='
		} else if i == filled && filled < barWidth {
			bar[i] = '>'
		} else {
			bar[i] = ' '
		}
	}

	unit := p.Units
	if unit == "" {
		unit = "B"
	}
	return fmt.Sprintf(" [%s] %s/%s", string(bar), formatSize(current, unit), formatSize(total, unit))
}

func formatSize(size int64, unit string) string {
	if unit != "bytes" && unit != "B" {
		return fmt.Sprintf("%d %s", size, unit)
	}
	const kb = 1024
	const mb = kb * 1024
	const gb = mb * 1024
	if size >= gb {
		return fmt.Sprintf("%.2f GB", float64(size)/gb)
	}
	if size >= mb {
		return fmt.Sprintf("%.2f MB", float64(size)/mb)
	}
	if size >= kb {
		return fmt.Sprintf("%.2f KB", float64(size)/kb)
	}
	return fmt.Sprintf("%d B", size)
}

// printBuildProgress decodes the JSON messages from the ImageBuildResponse and
// prints the build stream output to os.Stderr, returning an error if the build fails.
func printBuildProgress(ctx context.Context, body io.ReadCloser) error {
	defer func() { _ = body.Close() }()
	dec := json.NewDecoder(body)
	newLine := true
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var msg jsonstream.Message
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode: %w", err)
		}
		if msg.Error != nil {
			return fmt.Errorf("%s", msg.Error.Message)
		}
		if msg.Stream != "" {
			var buf bytes.Buffer
			for _, c := range []byte(msg.Stream) {
				if newLine {
					buf.WriteString("    ")
					newLine = false
				}
				buf.WriteByte(c)
				if c == '\n' {
					newLine = true
				}
			}
			_, _ = fmt.Fprint(progressOut, buf.String())
		}
	}
}
