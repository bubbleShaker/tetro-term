package game

import (
	"strings"
	"testing"
)

// テストの中で盤面を目で読める絵として書くためのヘルパ。
//
// これは internal/render の代わりではない。コアのテストがレンダラを呼ぶと、
// 下の層が上の層に依存することになり、docs/adr/0001 が引いた境界がテスト経由で崩れる。
// ここにあるのは色も枠も持たない、テスト専用の素朴な文字起こしである。

// emptyRune は空のセル。埋まったセルはミノの種類の頭文字（I O T S Z J L）で表す。
const emptyRune = '.'

// parseBoard は ASCII の絵からロック済みの盤面を作る。
//
// 書いた行は盤面の**下端にそろえて**置かれる。積み上がりを試すテストで、
// 上の空っぽな行を毎回 20 行ぶん書かずに済むようにするため。
func parseBoard(t *testing.T, art string) Board {
	t.Helper()

	var b Board
	rows := artRows(art)
	if len(rows) > Height {
		t.Fatalf("盤面の絵が %d 行ある（盤面は %d 行）", len(rows), Height)
	}
	top := Height - len(rows)

	for i, row := range rows {
		cols := []rune(row)
		if len(cols) != Width {
			t.Fatalf("盤面の絵の %d 行目が %d 列ある（盤面は %d 列）: %q", i+1, len(cols), Width, row)
		}
		for x, r := range cols {
			if r == emptyRune {
				continue
			}
			kind, ok := kindFromRune(r)
			if !ok {
				t.Fatalf("盤面の絵に知らない文字 %q がある", r)
			}
			b.cells[top+i][x] = Cell{Filled: true, Kind: kind}
		}
	}
	return b
}

// boardArt は盤面を parseBoard と同じ表記へ戻す。
//
// 上端の空行は落とす。盤面の下端はいつも同じ位置にあるので、こうしても
// セルの位置関係は失われず、期待値と実際の値が同じ形の文字列になる。
func boardArt(b *Board) string {
	empty := strings.Repeat(string(emptyRune), Width)

	lines := make([]string, 0, Height)
	for y := 0; y < Height; y++ {
		var sb strings.Builder
		for x := 0; x < Width; x++ {
			if c := b.cells[y][x]; c.Filled {
				sb.WriteString(c.Kind.String())
			} else {
				sb.WriteRune(emptyRune)
			}
		}
		lines = append(lines, sb.String())
	}
	for len(lines) > 0 && lines[0] == empty {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

// artRows は絵を行へ割り、インデントと空行を落とす。
func artRows(art string) []string {
	var rows []string
	for _, line := range strings.Split(art, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			rows = append(rows, line)
		}
	}
	return rows
}

func kindFromRune(r rune) (MinoKind, bool) {
	for _, k := range AllKinds() {
		if rune(k.String()[0]) == r {
			return k, true
		}
	}
	return 0, false
}

// fixedDraw は決まった順にミノを返す抽選（→ CONTEXT.md「抽選」）。
// 尽きたら最後のものを返し続ける。テストが「次に来るミノ」を決め打ちするために使う。
func fixedDraw(kinds ...MinoKind) func() MinoKind {
	i := 0
	return func() MinoKind {
		k := kinds[i]
		if i < len(kinds)-1 {
			i++
		}
		return k
	}
}

// dropUntilLocked はアクティブミノがロックされるまでソフトドロップし続ける。
// ロックされたかどうかは、次のミノに入れ替わって位置が上へ戻ったことで判る。
func dropUntilLocked(t *testing.T, g *Game) {
	t.Helper()

	before, ok := g.Active()
	if !ok {
		t.Fatal("すでにゲームオーバーになっている")
	}
	for i := 0; i < Height+boxSize; i++ {
		g.Handle(SoftDrop)
		if now, ok := g.Active(); !ok || now.Pos.Y <= before.Pos.Y {
			return
		}
	}
	t.Fatal("ソフトドロップを繰り返してもミノがロックされなかった")
}
