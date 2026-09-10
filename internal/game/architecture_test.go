package game

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// allowedImports はコアが依存してよいものの全部。
//
// コアは端末もブラウザも時計も知らない、というのがこのプロジェクトの背骨である
// （→ docs/adr/0001）。約束をコメントで書いても守られないので、自分自身の import を
// テストで見張る。ここに syscall/js や golang.org/x/term や internal/render が
// 現れたら、設計が崩れたということであり、追加するのではなく設計を直す。
//
// time が入っているのは型（time.Duration）のためだけで、いま何時かは問い合わせない。
var allowedImports = map[string]bool{
	"fmt":  true,
	"time": true,
}

func TestCoreDependsOnNothingEnvironmental(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("パッケージのディレクトリを読めない: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++

		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s を読めない: %v", name, err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s の import が読めない: %v", name, spec.Path.Value)
			}
			if !allowedImports[path] {
				t.Errorf("%s がコアの外のもの %q に依存している（%s）",
					name, path, fset.Position(spec.Pos()))
			}
		}
	}

	// 1 つも読めていないのに通る、という空振りを防ぐ。テストの置き場所が変わったり
	// ファイルの数え方を間違えたりすると、この見張りは黙って無力になる。
	if checked == 0 {
		t.Fatal("コアのファイルを 1 つも検査できていない")
	}
}
