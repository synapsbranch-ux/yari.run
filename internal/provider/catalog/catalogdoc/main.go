// Command catalogdoc prints the Provider Catalog as a Markdown table. The README embeds its output
// between the catalog-table markers; a test keeps them in sync. Run it with: make catalog-table.
package main

import (
	"fmt"

	"github.com/synapsbranch-ux/yari.run/internal/provider/catalog"
)

func main() { fmt.Print(catalog.MustLoad().Markdown()) }
