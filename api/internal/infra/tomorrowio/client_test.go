package tomorrowio

import (
	"context"
	"strings"
	"testing"
)

func TestDoError_DoesNotLeakAPIKey(t *testing.T) {
	const key = "leak-canary"
	c := NewClient(key, "http://127.0.0.1:1") // nothing listens; Do fails with *url.Error
	for name, fn := range map[string]func() error{
		"FetchPoint":    func() error { _, err := c.FetchPoint(context.Background(), 35.6, 139.7); return err },
		"FetchMinutely": func() error { _, err := c.FetchMinutely(context.Background(), 35.6, 139.7); return err },
	} {
		err := fn()
		if err == nil {
			t.Fatalf("%s: expected error", name)
		}
		if strings.Contains(err.Error(), key) {
			t.Errorf("%s: error leaks API key: %v", name, err)
		}
	}
}
