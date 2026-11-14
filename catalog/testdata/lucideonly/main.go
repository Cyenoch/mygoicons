// Command lucideonly lists one collection; its test checks what is linked.
package main

import (
	"fmt"

	"github.com/Cyenoch/mygoicons/catalog"
)

func main() { fmt.Println(len(catalog.Lucide().Icons)) }
