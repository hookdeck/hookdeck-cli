package slug

import (
	"github.com/gosimple/slug"
)

func Make(s string) string {
	slug.Lowercase = false
	return slug.Make(s)
}
