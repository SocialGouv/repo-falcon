package extract

import (
	"reflect"
	"testing"
)

func TestExtractPythonFile_Imports(t *testing.T) {
	src := []byte(`import os
import os.path
import a.b as c
from x.y import z
from typing import List, Optional
`)
	pf, err := ExtractPythonFile("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.b", "os", "os.path", "typing", "x.y"}
	if !reflect.DeepEqual(pf.Imports, want) {
		t.Fatalf("imports: got %v, want %v", pf.Imports, want)
	}
}

func TestExtractPythonFile_RelativeImports(t *testing.T) {
	src := []byte(`from . import util
from .util import double
from ..models import User
from ...pkg.sub import thing
`)
	pf, err := ExtractPythonFile("app/views.py", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".", "...pkg.sub", "..models", ".util"}
	if !reflect.DeepEqual(pf.Imports, want) {
		t.Fatalf("imports: got %v, want %v", pf.Imports, want)
	}
}

func TestExtractPythonFile_FutureImport(t *testing.T) {
	src := []byte(`from __future__ import annotations
import os
`)
	pf, err := ExtractPythonFile("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"__future__", "os"}
	if !reflect.DeepEqual(pf.Imports, want) {
		t.Fatalf("imports: got %v, want %v", pf.Imports, want)
	}
}

func TestExtractPythonFile_MultiImport(t *testing.T) {
	src := []byte(`import a, b, c
from os import (
    path,
    getcwd
)
`)
	pf, err := ExtractPythonFile("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c", "os"}
	if !reflect.DeepEqual(pf.Imports, want) {
		t.Fatalf("imports: got %v, want %v", pf.Imports, want)
	}
}

func TestExtractPythonFile_Symbols(t *testing.T) {
	src := []byte(`def handler(request):
    pass

class UserService:
    pass

async def fetch_data():
    pass
`)
	pf, err := ExtractPythonFile("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(pf.Symbols) != 3 {
		t.Fatalf("got %d symbols, want 3: %v", len(pf.Symbols), pf.Symbols)
	}
	// Symbols should be sorted by position.
	tests := []struct {
		kind string
		name string
	}{
		{"func", "handler"},
		{"class", "UserService"},
		{"func", "fetch_data"},
	}
	for i, tt := range tests {
		if pf.Symbols[i].Kind != tt.kind || pf.Symbols[i].Name != tt.name {
			t.Errorf("symbol[%d]: got %s %q, want %s %q", i, pf.Symbols[i].Kind, pf.Symbols[i].Name, tt.kind, tt.name)
		}
		if pf.Symbols[i].Language != "python" {
			t.Errorf("symbol[%d]: language = %q, want python", i, pf.Symbols[i].Language)
		}
	}
}

func TestExtractPythonFile_DecoratedSymbols(t *testing.T) {
	src := []byte(`@app.route("/")
def index():
    pass

@dataclass
class Config:
    debug: bool = False
`)
	pf, err := ExtractPythonFile("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(pf.Symbols) != 2 {
		t.Fatalf("got %d symbols, want 2: %v", len(pf.Symbols), pf.Symbols)
	}
	if pf.Symbols[0].Kind != "func" || pf.Symbols[0].Name != "index" {
		t.Errorf("symbol[0]: got %s %q, want func index", pf.Symbols[0].Kind, pf.Symbols[0].Name)
	}
	if pf.Symbols[1].Kind != "class" || pf.Symbols[1].Name != "Config" {
		t.Errorf("symbol[1]: got %s %q, want class Config", pf.Symbols[1].Kind, pf.Symbols[1].Name)
	}
}

func TestExtractPythonFile_CommentNotExtracted(t *testing.T) {
	src := []byte(`# import fake_module
# from bad import thing
import real_module
`)
	pf, err := ExtractPythonFile("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"real_module"}
	if !reflect.DeepEqual(pf.Imports, want) {
		t.Fatalf("imports: got %v, want %v", pf.Imports, want)
	}
}

func TestExtractPythonFile_SymbolPositions(t *testing.T) {
	src := []byte(`def hello():
    pass
`)
	pf, err := ExtractPythonFile("app.py", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(pf.Symbols) != 1 {
		t.Fatalf("got %d symbols, want 1", len(pf.Symbols))
	}
	s := pf.Symbols[0]
	// Position should be 1-indexed.
	if s.StartLine != 1 || s.StartCol != 1 {
		t.Errorf("start: got line %d col %d, want line 1 col 1", s.StartLine, s.StartCol)
	}
	if s.EndLine < 2 {
		t.Errorf("end line: got %d, want >= 2", s.EndLine)
	}
}
