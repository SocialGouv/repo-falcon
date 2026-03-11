package extract

import (
	"reflect"
	"testing"
)

func TestExtractJavaFile_PackageAndImports(t *testing.T) {
	src := []byte(`package com.example.app;

import java.util.List;
import java.util.Map;
import static java.lang.Math.*;
import com.example.model.User;
`)
	jf, err := ExtractJavaFile("App.java", src)
	if err != nil {
		t.Fatal(err)
	}
	if jf.PackageName != "com.example.app" {
		t.Fatalf("package: got %q, want %q", jf.PackageName, "com.example.app")
	}
	wantImports := []string{
		"com.example.model.User",
		"java.lang.Math.*",
		"java.util.List",
		"java.util.Map",
	}
	if !reflect.DeepEqual(jf.Imports, wantImports) {
		t.Fatalf("imports: got %v, want %v", jf.Imports, wantImports)
	}
}

func TestExtractJavaFile_ClassSymbol(t *testing.T) {
	src := []byte(`package com.example;

public class App {
    public static void main(String[] args) {
    }
}
`)
	jf, err := ExtractJavaFile("App.java", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(jf.Symbols) < 1 {
		t.Fatalf("got %d symbols, want >= 1", len(jf.Symbols))
	}
	// First symbol should be the class.
	if jf.Symbols[0].Kind != "class" || jf.Symbols[0].Name != "App" {
		t.Errorf("symbol[0]: got %s %q, want class App", jf.Symbols[0].Kind, jf.Symbols[0].Name)
	}
	if jf.Symbols[0].Language != "java" {
		t.Errorf("language: got %q, want java", jf.Symbols[0].Language)
	}
}

func TestExtractJavaFile_MethodsAndFields(t *testing.T) {
	src := []byte(`package com.example;

public class User {
    private String name;
    private int age;

    public User(String name, int age) {
        this.name = name;
        this.age = age;
    }

    public String getName() {
        return name;
    }

    public void setAge(int age) {
        this.age = age;
    }
}
`)
	jf, err := ExtractJavaFile("User.java", src)
	if err != nil {
		t.Fatal(err)
	}

	// Expect: class User, fields name + age, constructor User, methods getName + setAge.
	wantSymbols := []struct {
		kind          string
		name          string
		qualifiedName string
	}{
		{"class", "User", "User"},
		{"field", "name", "User.name"},
		{"field", "age", "User.age"},
		{"constructor", "User", "User.User"},
		{"method", "getName", "User.getName"},
		{"method", "setAge", "User.setAge"},
	}

	if len(jf.Symbols) != len(wantSymbols) {
		t.Fatalf("got %d symbols, want %d:\n", len(jf.Symbols), len(wantSymbols))
	}

	for i, ws := range wantSymbols {
		s := jf.Symbols[i]
		if s.Kind != ws.kind || s.Name != ws.name || s.QualifiedName != ws.qualifiedName {
			t.Errorf("symbol[%d]: got {%s %q %q}, want {%s %q %q}",
				i, s.Kind, s.Name, s.QualifiedName, ws.kind, ws.name, ws.qualifiedName)
		}
	}
}

func TestExtractJavaFile_InterfaceAndEnum(t *testing.T) {
	src := []byte(`package com.example;

public interface Greeter {
    void greet(String name);
}

public enum Color {
    RED, GREEN, BLUE
}
`)
	jf, err := ExtractJavaFile("Types.java", src)
	if err != nil {
		t.Fatal(err)
	}

	// At least the interface and enum should be found.
	foundInterface := false
	foundEnum := false
	for _, s := range jf.Symbols {
		if s.Kind == "interface" && s.Name == "Greeter" {
			foundInterface = true
		}
		if s.Kind == "enum" && s.Name == "Color" {
			foundEnum = true
		}
	}
	if !foundInterface {
		t.Error("missing interface Greeter")
	}
	if !foundEnum {
		t.Error("missing enum Color")
	}
}

func TestExtractJavaFile_CommentNotExtracted(t *testing.T) {
	src := []byte(`package com.example;

// import nope.Fake;
/* import nope.Fake2; */
import real.Package;
`)
	jf, err := ExtractJavaFile("App.java", src)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"real.Package"}
	if !reflect.DeepEqual(jf.Imports, want) {
		t.Fatalf("imports: got %v, want %v", jf.Imports, want)
	}
}

func TestExtractJavaFile_NoPackage(t *testing.T) {
	src := []byte(`import java.util.List;

class Simple {
}
`)
	jf, err := ExtractJavaFile("Simple.java", src)
	if err != nil {
		t.Fatal(err)
	}
	if jf.PackageName != "" {
		t.Fatalf("package: got %q, want empty", jf.PackageName)
	}
	if len(jf.Imports) != 1 || jf.Imports[0] != "java.util.List" {
		t.Fatalf("imports: got %v, want [java.util.List]", jf.Imports)
	}
}

func TestExtractJavaFile_SymbolPositions(t *testing.T) {
	src := []byte(`package com.example;

public class App {
}
`)
	jf, err := ExtractJavaFile("App.java", src)
	if err != nil {
		t.Fatal(err)
	}
	if len(jf.Symbols) < 1 {
		t.Fatalf("got 0 symbols")
	}
	s := jf.Symbols[0]
	// Position should be 1-indexed.
	if s.StartLine < 1 || s.StartCol < 1 {
		t.Errorf("start: got line %d col %d, want >= 1", s.StartLine, s.StartCol)
	}
}
