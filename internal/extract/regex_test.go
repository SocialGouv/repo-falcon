package extract

import (
	"reflect"
	"testing"
)

func TestExtractJSImportTargets(t *testing.T) {
	src := []byte(`// comment import x from 'nope'
import x from "react";
import {y} from 'z';
import 'side';
const fs = require('fs');
`)
	got := ExtractJSImportTargets(src)
	want := []string{"fs", "react", "side", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestExtractPythonImportTargets(t *testing.T) {
	src := []byte(`# comment: import no
import os
import a.b as c
from x.y import z
`)
	got := ExtractPythonImportTargets(src)
	want := []string{"a.b", "os", "x.y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestExtractJavaImportTargets(t *testing.T) {
	src := []byte(`import java.util.List;
import static java.lang.Math.*;
// import nope.X;
`)
	got := ExtractJavaImportTargets(src)
	want := []string{"java.lang.Math.*", "java.util.List"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
