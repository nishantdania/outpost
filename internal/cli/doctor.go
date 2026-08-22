package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func doctor(ctx context.Context, w io.Writer) error {
	r, e := request(ctx, http.MethodGet, "/doctor")
	if e != nil {
		return e
	}
	defer r.Body.Close()
	var b struct {
		Checks []struct {
			Name    string `json:"name"`
			OK      bool   `json:"ok"`
			Message string `json:"message"`
		} `json:"checks"`
	}
	if e = json.NewDecoder(r.Body).Decode(&b); e != nil {
		return e
	}
	failed := false
	for _, c := range b.Checks {
		mark := "✓"
		if !c.OK {
			mark = "✗"
			failed = true
		}
		fmt.Fprintf(w, "%s %s: %s\n", mark, c.Name, c.Message)
	}
	if failed {
		return fmt.Errorf("server is not ready")
	}
	return nil
}
