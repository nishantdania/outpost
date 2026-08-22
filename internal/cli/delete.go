package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func deleteOutpost(ctx context.Context, id string, stdout io.Writer) error {
	response, err := request(ctx, http.MethodDelete, "/outposts/"+url.PathEscape(id))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	fmt.Fprintf(stdout, "Deleted %s\n", id)
	return nil
}
