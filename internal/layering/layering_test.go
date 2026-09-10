// Package layering はパッケージ同士の依存の向きをテストで見張る。
//
// このプロジェクトの背骨は「コアは環境を知らない」「ANSI を組み立てるのは
// レンダラだけ」という 2 つの約束である（→ docs/adr/0001）。約束はコメントで
// 書いても守られないので、各パッケージが何を import しているかをここで確かめる。
//
// 見張りを層ごとに分けて置くと同じ仕組みを何度も書くことになるので、
// **依存の向きに関する決まりはこの 1 ファイルにまとめてある**。
// 新しい層を足したときは、ここに 1 行足すのを忘れないこと。
package layering

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// rule は 1 つのパッケージが依存してよいものの全部。ここに無いものを import したら落ちる。
type rule struct {
	dir     string
	why     string
	allowed []string
}

var rules = []rule{
	{
		dir: "../game",
		why: "コアは端末もブラウザも時計も知らない（→ docs/adr/0001）。" +
			"ここに syscall/js や golang.org/x/term や internal/render が現れたら、" +
			"追加するのではなく設計を直す。time は型のためだけで、いま何時かは問い合わせない",
		allowed: []string{"fmt", "time"},
	},
	{
		dir: "../input",
		why: "キーの読み替えは端末に固有ではない。xterm.js が本物の端末と同じバイト列を" +
			"送ってくるので、M2 のブラウザ版はここをそのまま使い回す。" +
			"golang.org/x/term のような端末専用のものが現れたら、その前提が壊れる",
		allowed: []string{
			"github.com/bubbleShaker/tetro-term/internal/game",
		},
	},
	{
		dir: "../render",
		why: "レンダラは盤面を文字列に直すだけで、その文字列をどこへ流すかは知らない。" +
			"端末やブラウザに触るものが現れたら、CLI 版とブラウザ版で同じ画が出る保証が崩れる。" +
			"コアに依存してよいが、逆向きは上の決まりが禁じている",
		allowed: []string{
			"fmt",
			"strings",
			"github.com/bubbleShaker/tetro-term/internal/game",
		},
	},
}

func TestPackagesOnlyDependOnWhatTheyAreAllowedTo(t *testing.T) {
	for _, r := range rules {
		t.Run(filepath.Base(r.dir), func(t *testing.T) {
			allowed := map[string]bool{}
			for _, path := range r.allowed {
				allowed[path] = true
			}

			checked := 0
			for _, name := range sourceFiles(t, r.dir) {
				checked++
				for _, path := range importsOf(t, filepath.Join(r.dir, name)) {
					if !allowed[path] {
						t.Errorf("%s が %q に依存している\n理由: %s", name, path, r.why)
					}
				}
			}

			// 1 つも読めていないのに通る、という空振りを防ぐ。パッケージの置き場所が
			// 変わったりファイルの数え方を間違えたりすると、この見張りは黙って無力になる。
			if checked == 0 {
				t.Fatalf("%s のファイルを 1 つも検査できていない", r.dir)
			}
		})
	}
}

// sourceFiles はテスト以外の .go ファイルの名前を返す。
// テストは何に依存してもよい（テスト用の道具立てまで縛ると窮屈すぎる）。
func sourceFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("%s を読めない: %v", dir, err)
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	return names
}

func importsOf(t *testing.T, path string) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("%s を読めない: %v", path, err)
	}

	var paths []string
	for _, spec := range file.Imports {
		quoted, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("%s の import が読めない: %v", path, spec.Path.Value)
		}
		paths = append(paths, quoted)
	}
	return paths
}
