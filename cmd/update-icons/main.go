// Command update-icons maintains the bundled npm inputs. Ordinary generation is offline.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Cyenoch/mygoicons/internal/maint"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "update-icons:", err)
		os.Exit(1)
	}
}
func run() error {
	root := flag.String("root", ".", "repository root")
	lucide := flag.String("lucide", "", "exact Lucide version; empty queries latest")
	twemoji := flag.String("twemoji", "", "exact Twemoji version; empty queries latest")
	list := flag.String("versions", "", "list available versions: lucide or twemoji")
	check := flag.Bool("check", false, "preview only; never write")
	accept := flag.Bool("accept-license", false, "accept reviewed license and notice changes")
	recover := flag.Bool("recover", false, "restore an interrupted transaction")
	summary := flag.String("summary", "", "also write the review summary to this file")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *recover {
		return maint.Recover(*root)
	}
	u := maint.New(*root)
	if *list != "" {
		v, err := u.Versions(context.Background(), *list)
		if err != nil {
			return err
		}
		fmt.Println(strings.Join(v, "\n"))
		return nil
	}
	changes, report, err := u.Plan(context.Background(), map[string]string{"lucide": *lucide, "twemoji": *twemoji}, *accept)
	fmt.Print(report)
	if *summary != "" {
		if writeErr := os.WriteFile(*summary, []byte(report), 0o644); writeErr != nil {
			return writeErr
		}
	}
	if err != nil {
		return err
	}
	fmt.Printf("%d changed files\n", len(changes))
	if *check {
		return nil
	}
	return maint.Apply(*root, changes)
}
