package client

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
)

func (c *Client) BuildImage(ctx context.Context, tag, dir string) (outpost.Image, error) {
	file, err := os.CreateTemp("", "outpost-context-")
	if err != nil {
		return outpost.Image{}, err
	}
	name := file.Name()
	defer os.Remove(name)
	if err = tarDirectory(file, dir); err != nil {
		file.Close()
		return outpost.Image{}, err
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return outpost.Image{}, err
	}
	defer file.Close()
	response, err := c.api.BuildImageWithBodyWithResponse(ctx, &api.BuildImageParams{Tag: tag}, "application/x-tar", file)
	if err != nil {
		return outpost.Image{}, err
	}
	if response.JSON201 == nil {
		return outpost.Image{}, fmt.Errorf("build image: %s", response.Status())
	}
	return image(*response.JSON201), nil
}
func tarDirectory(w io.Writer, dir string) error {
	base, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(w)
	defer tw.Close()
	var entries int
	var total int64
	return filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		entries++
		if entries > 10000 {
			return fmt.Errorf("build context has too many entries")
		}
		if err != nil {
			return err
		}
		if path == base {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("build context contains symlink")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(name)
		if len(h.Name) > 1024 || (info.Mode().IsRegular() && info.Size() > 32<<20) {
			return fmt.Errorf("build context entry exceeds limit")
		}
		if info.Mode().IsRegular() {
			total += info.Size()
			if total > 64<<20 {
				return fmt.Errorf("build context exceeds limit")
			}
		}
		if err = tw.WriteHeader(h); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		f.Close()
		return err
	})
}
func (c *Client) ImportImage(ctx context.Context, tag string, input io.Reader) (outpost.Image, error) {
	response, err := c.api.ImportImageWithBodyWithResponse(ctx, &api.ImportImageParams{Tag: tag}, "application/octet-stream", input)
	if err != nil {
		return outpost.Image{}, err
	}
	if response.JSON201 == nil {
		return outpost.Image{}, fmt.Errorf("import image: %s", response.Status())
	}
	return image(*response.JSON201), nil
}
func (c *Client) ListImages(ctx context.Context) ([]outpost.Image, error) {
	response, err := c.api.ListImagesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("list images: %s", response.Status())
	}
	out := make([]outpost.Image, 0, len(*response.JSON200))
	for _, v := range *response.JSON200 {
		out = append(out, image(v))
	}
	return out, nil
}
func (c *Client) GetImage(ctx context.Context, ref string) (outpost.Image, error) {
	response, err := c.api.GetImageWithResponse(ctx, ref)
	if err != nil {
		return outpost.Image{}, err
	}
	if response.JSON200 == nil {
		return outpost.Image{}, fmt.Errorf("inspect image: %s", response.Status())
	}
	return image(*response.JSON200), nil
}
func (c *Client) RemoveImage(ctx context.Context, ref string) error {
	response, err := c.api.DeleteImageWithResponse(ctx, ref)
	if err != nil {
		return err
	}
	if response.StatusCode() != 204 {
		return fmt.Errorf("remove image: %s", response.Status())
	}
	return nil
}
func (c *Client) GCImages(ctx context.Context) ([]string, error) {
	response, err := c.api.GcImagesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("GC images: %s", response.Status())
	}
	return *response.JSON200, nil
}
func image(v api.Image) outpost.Image {
	return outpost.Image{Digest: v.Digest, Size: int64(v.SizeBytes), Tags: v.Tags, CreatedAt: v.CreatedAt}
}
