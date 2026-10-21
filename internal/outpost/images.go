package outpost

import "github.com/nishantdania/outpost/internal/catalog"

// Image metadata belongs to the catalog. These aliases keep the transport model
// stable while callers migrate to the catalog vocabulary.
type Image = catalog.Image

var (
	ErrImageNotFound = catalog.ErrImageNotFound
	ErrInvalidImage  = catalog.ErrInvalidImage
)

func ValidDigest(value string) bool   { return catalog.ValidDigest(value) }
func ValidImageTag(value string) bool { return catalog.ValidImageTag(value) }
