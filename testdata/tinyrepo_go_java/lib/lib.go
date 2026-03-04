package lib

import "strings"

type Thing struct{ Name string }

func (t Thing) Upper() string { return strings.ToUpper(t.Name) }
