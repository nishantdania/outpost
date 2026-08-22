package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func list(ctx context.Context, stdout io.Writer) error {
	response, err := request(ctx, http.MethodGet, "/outposts")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	var body struct {
		Outposts []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
			IP     string `json:"ip"`
		} `json:"outposts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return err
	}
	for _, record := range body.Outposts {
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", record.ID, record.Name, record.Status, record.IP)
	}
	return nil
}
