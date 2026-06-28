package extract

import (
	"sort"
	"testing"
)

func calleesFrom(refs []Reference, from string) []string {
	var out []string
	for _, r := range refs {
		if r.FromQualified == from {
			out = append(out, r.Callee)
		}
	}
	sort.Strings(out)
	return out
}

func hasAll(got, want []string) bool {
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func TestGoReferences(t *testing.T) {
	src := []byte(`package p
func A() { B(); helper(); pkg.Ext() }
func B() {}
func helper() {}
`)
	gf, err := ExtractGoFile("p.go", src)
	if err != nil {
		t.Fatal(err)
	}
	got := calleesFrom(gf.References, "A")
	if !hasAll(got, []string{"B", "helper", "Ext"}) {
		t.Errorf("A callees = %v, want B, helper, Ext", got)
	}
}

func TestJSReferences(t *testing.T) {
	src := []byte(`export function a() { b(); obj.c(); }
function b() {}
`)
	jf, err := ExtractJSFile("a.js", src, "js")
	if err != nil {
		t.Fatal(err)
	}
	got := calleesFrom(jf.References, "a")
	if !hasAll(got, []string{"b", "c"}) {
		t.Errorf("a callees = %v, want b, c", got)
	}
}

func TestPythonReferences(t *testing.T) {
	src := []byte(`def a():
    b()
    x.c()

def b():
    pass
`)
	pf, err := ExtractPythonFile("a.py", src)
	if err != nil {
		t.Fatal(err)
	}
	got := calleesFrom(pf.References, "a")
	if !hasAll(got, []string{"b", "c"}) {
		t.Errorf("a callees = %v, want b, c", got)
	}
}

func recvTypeOf(refs []Reference, from, callee string) (string, bool) {
	for _, r := range refs {
		if r.FromQualified == from && r.Callee == callee {
			return r.RecvType, true
		}
	}
	return "", false
}

func TestJavaReceiverTyping(t *testing.T) {
	src := []byte(`package p;
class C {
    void a() {
        this.b();
        D d = new D();
        d.run();
    }
    void b() {}
}
`)
	jf, err := ExtractJavaFile("C.java", src)
	if err != nil {
		t.Fatal(err)
	}
	// this.b() -> receiver is the enclosing class C.
	if rt, ok := recvTypeOf(jf.References, "C.a", "b"); !ok || rt != "C" {
		t.Errorf("this.b() RecvType = %q (ok=%v), want C", rt, ok)
	}
	// d.run() where `D d = new D()` -> receiver typed D.
	if rt, ok := recvTypeOf(jf.References, "C.a", "run"); !ok || rt != "D" {
		t.Errorf("d.run() RecvType = %q (ok=%v), want D", rt, ok)
	}
}

func TestPythonReceiverTyping(t *testing.T) {
	src := []byte(`class C:
    def a(self):
        self.b()

    def b(self):
        pass
`)
	pf, err := ExtractPythonFile("c.py", src)
	if err != nil {
		t.Fatal(err)
	}
	if rt, ok := recvTypeOf(pf.References, "C.a", "b"); !ok || rt != "C" {
		t.Errorf("self.b() RecvType = %q (ok=%v), want C", rt, ok)
	}
}

func TestJavaReferences(t *testing.T) {
	src := []byte(`package p;
class C {
    void a() { b(); new D(); }
    void b() {}
}
`)
	jf, err := ExtractJavaFile("C.java", src)
	if err != nil {
		t.Fatal(err)
	}
	// Calls inside C.a() attribute to the innermost enclosing symbol "C.a".
	got := calleesFrom(jf.References, "C.a")
	if !hasAll(got, []string{"b", "D"}) {
		t.Errorf("C.a callees = %v, want b, D", got)
	}
}
