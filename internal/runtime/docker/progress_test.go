package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/jsonstream"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		unit     string
		expected string
	}{
		{512, "B", "512 B"},
		{512, "bytes", "512 B"},
		{1024, "B", "1.00 KB"},
		{1536, "B", "1.50 KB"},
		{1024 * 1024, "B", "1.00 MB"},
		{1024 * 1024 * 1024, "B", "1.00 GB"},
		{50, "items", "50 items"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%d_%s", tc.size, tc.unit), func(t *testing.T) {
			got := formatSize(tc.size, tc.unit)
			if got != tc.expected {
				t.Errorf("formatSize(%d, %q) = %q; want %q", tc.size, tc.unit, got, tc.expected)
			}
		})
	}
}

func TestFormatProgress(t *testing.T) {
	tests := []struct {
		name     string
		p        *jsonstream.Progress
		expected string
	}{
		{
			name:     "nil progress",
			p:        nil,
			expected: "",
		},
		{
			name:     "total <= 0",
			p:        &jsonstream.Progress{Current: 50, Total: 0},
			expected: "",
		},
		{
			name:     "current > total",
			p:        &jsonstream.Progress{Current: 120, Total: 100, Units: "B"},
			expected: " [====================] 100 B/100 B",
		},
		{
			name:     "50 percent with B",
			p:        &jsonstream.Progress{Current: 50, Total: 100, Units: "B"},
			expected: " [==========>         ] 50 B/100 B",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatProgress(tc.p)
			if got != tc.expected {
				t.Errorf("formatProgress() = %q; want %q", got, tc.expected)
			}
		})
	}
}

type mockImagePullResponse struct {
	messages []jsonstream.Message
	err      error
}

func (m *mockImagePullResponse) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (m *mockImagePullResponse) Close() error {
	return nil
}

func (m *mockImagePullResponse) JSONMessages(ctx context.Context) iter.Seq2[jsonstream.Message, error] {
	return func(yield func(jsonstream.Message, error) bool) {
		for _, msg := range m.messages {
			if !yield(msg, nil) {
				return
			}
		}
		if m.err != nil {
			yield(jsonstream.Message{}, m.err)
		}
	}
}

func (m *mockImagePullResponse) Wait(ctx context.Context) error {
	return nil
}

func TestPrintPullProgress(t *testing.T) {
	// Backup package-level variables
	oldProgressOut := progressOut
	oldProgressFd := progressFd
	oldIsTerminalFn := isTerminalFn
	defer func() {
		progressOut = oldProgressOut
		progressFd = oldProgressFd
		isTerminalFn = oldIsTerminalFn
	}()

	t.Run("Non-Terminal Output", func(t *testing.T) {
		var buf bytes.Buffer
		progressOut = &buf
		isTerminalFn = func(fd uintptr) bool { return false }

		resp := &mockImagePullResponse{
			messages: []jsonstream.Message{
				{ID: "layer1", Status: "Pulling fs layer"},
				{ID: "layer2", Status: "Pulling fs layer"},
				{ID: "layer1", Status: "Downloading"},
				{ID: "layer1", Status: "Downloading"}, // duplicate status, should be ignored
				{ID: "layer2", Status: "Downloading"},
				{ID: "layer1", Status: "Download complete"},
				{ID: "layer2", Status: "Download complete"},
				{ID: "", Status: "Digest: sha256:abc"},
				{ID: "", Status: ""}, // empty status/ID, should be ignored
				{ID: "", Status: "Status: Downloaded newer image"},
			},
		}

		err := printPullProgress(context.Background(), resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		expected := []string{
			"layer1      : Pulling fs layer\n",
			"layer2      : Pulling fs layer\n",
			"layer1      : Downloading\n",
			"layer2      : Downloading\n",
			"layer1      : Download complete\n",
			"layer2      : Download complete\n",
			"Digest: sha256:abc\n",
			"Status: Downloaded newer image\n",
		}

		for _, exp := range expected {
			if !strings.Contains(got, exp) {
				t.Errorf("expected output to contain %q, but got %q", exp, got)
			}
		}

		// Ensure the duplicate downloading message for layer1 was indeed ignored
		if strings.Count(got, "layer1      : Downloading\n") != 1 {
			t.Errorf("expected 'layer1      : Downloading' to be printed exactly once, got: %q", got)
		}

		// Ensure no empty lines or empty status prints occurred
		if strings.Contains(got, "      : \n") || strings.Contains(got, "\n\n") {
			t.Errorf("unexpected formatting or empty lines in output: %q", got)
		}
	})

	t.Run("Terminal Output with Progress Bar", func(t *testing.T) {
		var buf bytes.Buffer
		progressOut = &buf
		isTerminalFn = func(fd uintptr) bool { return true }

		resp := &mockImagePullResponse{
			messages: []jsonstream.Message{
				{ID: "layer1", Status: "Downloading", Progress: &jsonstream.Progress{Current: 50, Total: 100, Units: "B"}},
				{ID: "layer2", Status: "Downloading", Progress: &jsonstream.Progress{Current: 10, Total: 100, Units: "B"}},
				{ID: "", Status: "Digest: sha256:abc"},
				{ID: "layer1", Status: "Complete"},
			},
		}

		err := printPullProgress(context.Background(), resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		// Test escape sequences and output contents
		// It should allocate new lines with \n when layers are first registered:
		if strings.Count(got, "\n") < 3 {
			t.Errorf("expected at least 3 newlines for allocation and status printing, got: %q", got)
		}

		// It should contain ANSI escape codes for going up and down
		if !strings.Contains(got, "\033[1A") { // going up
			t.Errorf("expected output to contain up escape sequence, got: %q", got)
		}
		if !strings.Contains(got, "\033[1B\r") { // going down with carriage return
			t.Errorf("expected output to contain down/CR escape sequence, got: %q", got)
		}
		if !strings.Contains(got, "\033[3A") { // going up 3 lines when extraLines = 1 and updating layer1
			t.Errorf("expected output to contain up-3 escape sequence when extraLines is active, got: %q", got)
		}
	})

	t.Run("Error propagation", func(t *testing.T) {
		isTerminalFn = func(fd uintptr) bool { return false }
		expectedErr := errors.New("pull failure")
		resp := &mockImagePullResponse{
			err: expectedErr,
		}

		err := printPullProgress(context.Background(), resp)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}

func TestPrintBuildProgress(t *testing.T) {
	oldProgressOut := progressOut
	defer func() {
		progressOut = oldProgressOut
	}()

	var buf bytes.Buffer
	progressOut = &buf

	t.Run("Successful Build", func(t *testing.T) {
		buf.Reset()
		jsonStream := `{"stream":"Step 1/3 : FROM alpine\n"}
{"stream":" ---> d9e853e87e55\n"}`
		body := io.NopCloser(strings.NewReader(jsonStream))
		err := printBuildProgress(context.Background(), body)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "        Step 1/3 : FROM alpine\n         ---> d9e853e87e55\n"
		if buf.String() != expected {
			t.Errorf("expected %q, got %q", expected, buf.String())
		}
	})

	t.Run("Build Error", func(t *testing.T) {
		buf.Reset()
		jsonStream := `{"stream":"Step 1/3 : FROM alpine\n"}
{"errorDetail":{"message":"manifest not found"},"error":"manifest not found"}`
		body := io.NopCloser(strings.NewReader(jsonStream))
		err := printBuildProgress(context.Background(), body)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err.Error() != "manifest not found" {
			t.Errorf("expected 'manifest not found', got %v", err)
		}
		if buf.String() != "        Step 1/3 : FROM alpine\n" {
			t.Errorf("expected partial stream output, got %q", buf.String())
		}
	})

	t.Run("Context Cancelled", func(t *testing.T) {
		buf.Reset()
		jsonStream := `{"stream":"Step 1/3 : FROM alpine\n"}`
		body := io.NopCloser(strings.NewReader(jsonStream))

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // immediately cancel

		err := printBuildProgress(ctx, body)
		if err == nil {
			t.Fatalf("expected context cancelled error, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}
