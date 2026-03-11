package extract

import (
	"reflect"
	"testing"
)

func TestExtractJSFile_Imports(t *testing.T) {
	src := []byte(`// comment import x from 'nope'
import x from "react";
import {y} from 'z';
import 'side';
const fs = require('fs');
`)
	got, err := ExtractJSFile("test.js", src, "js")
	if err != nil {
		t.Fatalf("ExtractJSFile error: %v", err)
	}
	want := []string{"fs", "react", "side", "z"}
	if !reflect.DeepEqual(got.Imports, want) {
		t.Fatalf("imports: got %#v want %#v", got.Imports, want)
	}
}

func TestExtractJSFile_ReExports(t *testing.T) {
	src := []byte(`export { greet } from './lib.js'
export * from "./utils"
import React from 'react'
`)
	got, err := ExtractJSFile("index.js", src, "js")
	if err != nil {
		t.Fatalf("ExtractJSFile error: %v", err)
	}
	want := []string{"./lib.js", "./utils", "react"}
	if !reflect.DeepEqual(got.Imports, want) {
		t.Fatalf("imports: got %#v want %#v", got.Imports, want)
	}
}

func TestExtractJSFile_RequireNested(t *testing.T) {
	src := []byte(`function load() {
  const mod = require('dynamic-mod');
  return mod;
}
const top = require('top-level');
`)
	got, err := ExtractJSFile("loader.js", src, "js")
	if err != nil {
		t.Fatalf("ExtractJSFile error: %v", err)
	}
	want := []string{"dynamic-mod", "top-level"}
	if !reflect.DeepEqual(got.Imports, want) {
		t.Fatalf("imports: got %#v want %#v", got.Imports, want)
	}
}

func TestExtractJSFile_Symbols(t *testing.T) {
	src := []byte(`function greet(name) {
  return 'hello ' + name;
}

class MyClass {
  constructor() {}
}

const PI = 3.14;
let count = 0;
var legacy = true;
`)
	got, err := ExtractJSFile("symbols.js", src, "js")
	if err != nil {
		t.Fatalf("ExtractJSFile error: %v", err)
	}

	if len(got.Symbols) != 5 {
		t.Fatalf("expected 5 symbols, got %d: %+v", len(got.Symbols), got.Symbols)
	}

	names := make([]string, len(got.Symbols))
	for i, s := range got.Symbols {
		names[i] = s.Kind + ":" + s.Name
	}
	wantNames := []string{"function:greet", "class:MyClass", "const:PI", "let:count", "var:legacy"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("symbol names: got %#v want %#v", names, wantNames)
	}

	// Verify positions are 1-indexed.
	if got.Symbols[0].StartLine != 1 {
		t.Fatalf("expected greet at line 1, got %d", got.Symbols[0].StartLine)
	}
	if got.Symbols[0].StartCol != 1 {
		t.Fatalf("expected greet at col 1, got %d", got.Symbols[0].StartCol)
	}
}

func TestExtractJSFile_ExportedSymbols(t *testing.T) {
	src := []byte(`export function main() {}
export class App {}
export const VERSION = '1.0';
function helper() {}
`)
	got, err := ExtractJSFile("app.js", src, "js")
	if err != nil {
		t.Fatalf("ExtractJSFile error: %v", err)
	}

	// Should extract exported + non-exported symbols.
	if len(got.Symbols) != 4 {
		t.Fatalf("expected 4 symbols, got %d: %+v", len(got.Symbols), got.Symbols)
	}
}

func TestExtractJSFile_JSX(t *testing.T) {
	src := []byte(`import React from 'react';

export default function App() {
  return <div className="app">Hello</div>;
}
`)
	got, err := ExtractJSFile("App.jsx", src, "js")
	if err != nil {
		t.Fatalf("JSX parse error: %v", err)
	}
	if len(got.Imports) != 1 || got.Imports[0] != "react" {
		t.Fatalf("imports: got %#v want [react]", got.Imports)
	}
}

func TestExtractJSFile_TypeScript(t *testing.T) {
	src := []byte(`import { useState } from 'react';

interface Props {
  name: string;
}

export function Greeting(props: Props): JSX.Element {
  const [count, setCount] = useState<number>(0);
  return <div>{props.name}</div>;
}
`)
	got, err := ExtractJSFile("Greeting.tsx", src, "ts")
	if err != nil {
		t.Fatalf("TSX parse error: %v", err)
	}
	if len(got.Imports) != 1 || got.Imports[0] != "react" {
		t.Fatalf("imports: got %#v want [react]", got.Imports)
	}
	// Should have the exported function.
	found := false
	for _, s := range got.Symbols {
		if s.Name == "Greeting" && s.Kind == "function" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Greeting function symbol, got %+v", got.Symbols)
	}
}

func TestExtractJSFile_CommentNotExtracted(t *testing.T) {
	src := []byte(`// import fake from 'not-real'
/* import block from 'also-not-real' */
import real from 'real-module'
`)
	got, err := ExtractJSFile("test.js", src, "js")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got.Imports) != 1 || got.Imports[0] != "real-module" {
		t.Fatalf("imports: got %#v want [real-module]", got.Imports)
	}
}
