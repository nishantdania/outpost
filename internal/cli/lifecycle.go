package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nishantdania/outpost/internal/outpost"
)

func lifecycle(ctx context.Context, id, action string, stdout io.Writer) error {
	response, err := request(ctx, http.MethodPost, "/outposts/"+id+"/"+action)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", response.Status)
	}
	var record outpost.Record
	if err := json.NewDecoder(response.Body).Decode(&record); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "%s %s\n", map[string]string{"start": "Started", "stop": "Stopped"}[action], record.ID)
	return nil
}
