package greeter

import (
	"fmt"

	"example.com/tinyrepo/gojs/pkg/util"
)

// Greeter says hello.
type Greeter struct {
	Prefix string
}

func New(prefix string) *Greeter {
	return &Greeter{Prefix: prefix}
}

func (g *Greeter) Greet(name string) string {
	n := util.Add(len(g.Prefix), len(name))
	return fmt.Sprintf("%s %s (%d)", g.Prefix, name, n)
}
