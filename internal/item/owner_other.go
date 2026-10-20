//go:build !unix

package item

import (
	"io/fs"
	"os"
)

// keepOwner does nothing where files have no Unix owner.
func keepOwner(*os.File, fs.FileInfo) {}
