package game

import "testing"

func TestClearLinesRemovesFullRowsAndDropsAbove(t *testing.T) {
	b := parseBoard(t, `
		...T......
		IIIIIIIIII
		..J.......
		IIIIIIIIII
	`)

	if got := b.clearLines(); got != 2 {
		t.Errorf("消えた行数が %d、期待は 2", got)
	}

	// 埋まっていた 2 行が消え、残った 2 行が下端へ落ちる。上下の順は保たれる。
	want := parseBoard(t, `
		...T......
		..J.......
	`)
	if got, want := boardArt(&b), boardArt(&want); got != want {
		t.Errorf("消去後の盤面が違う\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestClearLinesLeavesIncompleteRowsAlone(t *testing.T) {
	art := `
		IIIIIIIII.
		.IIIIIIIII
	`
	b := parseBoard(t, art)
	before := boardArt(&b)

	if got := b.clearLines(); got != 0 {
		t.Errorf("消えた行数が %d、期待は 0", got)
	}
	if got := boardArt(&b); got != before {
		t.Errorf("1 マス欠けた行が動いてしまった\n--- got ---\n%s\n--- want ---\n%s", got, before)
	}
}

// At は表示する側が盤面を読むための唯一の入口になる（→ M1 の PR 2）。
// 盤面の外を指されたときに落ちるのではなく空のセルを返すのは、
// 枠の外まで一続きに描きたい呼び出し側が、境目を自分で気にせずに済むようにするため。
func TestAtReadsCellsAndTreatsOutsideAsEmpty(t *testing.T) {
	b := parseBoard(t, `
		....T.....
	`)

	if got := b.At(Point{X: 4, Y: Height - 1}); !got.Filled || got.Kind != T {
		t.Errorf("埋まっているセルが %+v として読めた", got)
	}
	if got := b.At(Point{X: 3, Y: Height - 1}); got.Filled {
		t.Errorf("空のセルが %+v として読めた", got)
	}

	outside := []Point{
		{X: -1, Y: 0},
		{X: Width, Y: 0},
		{X: 0, Y: -1},
		{X: 0, Y: Height},
	}
	for _, p := range outside {
		if got := b.At(p); got.Filled {
			t.Errorf("盤面の外 %+v が %+v として読めた", p, got)
		}
	}
}

func TestCollidesAtWallsAndFloor(t *testing.T) {
	var b Board

	tests := []struct {
		name string
		pos  Point
		want bool
	}{
		{"盤面の中", Point{X: 3, Y: 0}, false},
		{"左端に接する", Point{X: -1, Y: 0}, false},
		{"左端をはみ出す", Point{X: -2, Y: 0}, true},
		{"右端に接する", Point{X: Width - boxSize + 1, Y: 0}, false},
		{"右端をはみ出す", Point{X: Width - boxSize + 2, Y: 0}, true},
		{"床に接する", Point{X: 3, Y: Height - 2}, false},
		{"床を突き抜ける", Point{X: 3, Y: Height - 1}, true},
	}
	// 判定するのは箱ではなくセルなので、O が左端に接するときの Pos.X は -1 になる。
	// O は箱の 1〜2 列目を占めており、箱の 0 列目は空だからである。
	// 「はみ出している」のは箱ではなくセルだけを見て決まる。
	for _, tt := range tests {
		m := ActiveMino{Kind: O, Rot: Rot0, Pos: tt.pos}
		if got := b.collides(m); got != tt.want {
			t.Errorf("%s（%+v）: collides = %v、期待は %v", tt.name, tt.pos, got, tt.want)
		}
	}
}

func TestCollidesWithLockedCells(t *testing.T) {
	b := parseBoard(t, `
		....I.....
	`)

	// 埋まっているのは最下段の 4 列目。O が 4〜5 列目に降りてくると重なる。
	overlapping := ActiveMino{Kind: O, Rot: Rot0, Pos: Point{X: 3, Y: Height - 2}}
	if !b.collides(overlapping) {
		t.Error("ロック済みのセルと重なっているのに collides が false")
	}

	clear := ActiveMino{Kind: O, Rot: Rot0, Pos: Point{X: 5, Y: Height - 2}}
	if b.collides(clear) {
		t.Error("重なっていないのに collides が true")
	}
}

func TestLockWritesCellsWithTheirKind(t *testing.T) {
	var b Board
	b.lock(ActiveMino{Kind: T, Rot: Rot0, Pos: Point{X: 3, Y: Height - 2}})

	want := parseBoard(t, `
		....T.....
		...TTT....
	`)
	if got, want := boardArt(&b), boardArt(&want); got != want {
		t.Errorf("ロック後の盤面が違う\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
