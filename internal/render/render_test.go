package render

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

func TestFrameDrawsTheActiveMinoOnAnEmptyBoard(t *testing.T) {
	g := newGame(game.O)

	lines := boardLines(t, Frame(g))

	// O は 4〜5 列目に出る。1 セルが半角 2 文字なので、8〜11 文字目が塗られる。
	want := []string{
		"........####........",
		"........####........",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("%d 行目が\n  %q\n期待は\n  %q", i, lines[i], w)
		}
	}
	for i := len(want); i < len(lines); i++ {
		if strings.ContainsRune(lines[i], '#') {
			t.Errorf("何も無いはずの %d 行目が塗られている: %q", i, lines[i])
		}
	}
}

func TestFrameIsWrappedInAnAsciiBorder(t *testing.T) {
	g := newGame(game.O)

	lines := strings.Split(frameArt(Frame(g)), "\n")

	border := "+" + strings.Repeat("-", game.Width*cellWidth) + "+"
	if lines[0] != border {
		t.Errorf("上の枠が %q、期待は %q", lines[0], border)
	}
	if last := lines[len(lines)-1]; last != border {
		t.Errorf("下の枠が %q、期待は %q", last, border)
	}
	for i, line := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			t.Errorf("盤面 %d 行目が縦の枠に挟まれていない: %q", i, line)
		}
		// 全部の行の幅がそろっていること。ここが崩れるのは、端末によって幅の変わる
		// 文字（「█」や罫線素片）が混ざったときである。
		if len([]rune(line)) != len(border) {
			t.Errorf("盤面 %d 行目の幅が %d、期待は %d", i, len([]rune(line)), len(border))
		}
	}
}

// 盤面に積まれたセルと、いま操作しているミノの両方が同じ画に乗る。
// 盤面はロック済みしか持たない（→ CONTEXT.md）ので、重ね合わせは表示側の仕事である。
func TestFrameOverlaysTheActiveMinoOnTheLockedCells(t *testing.T) {
	g := newGame(game.O, game.O)
	lockOne(t, g)

	lines := boardLines(t, Frame(g))

	if got, want := lines[0], "........####........"; got != want {
		t.Errorf("新しいミノが上に出ていない: %q", got)
	}
	if got, want := lines[len(lines)-1], "........####........"; got != want {
		t.Errorf("ロック済みのセルが最下段に描かれていない: %q", got)
	}
}

func TestEachMinoKindGetsItsOwnColor(t *testing.T) {
	seen := map[int]game.MinoKind{}
	for _, kind := range game.AllKinds() {
		color, ok := minoColor[kind]
		if !ok {
			t.Fatalf("%v の色が決まっていない", kind)
		}
		if other, dup := seen[color]; dup {
			t.Errorf("%v と %v が同じ色 %d を使っている", kind, other, color)
		}
		seen[color] = kind

		frame := Frame(newGame(kind))
		if want := sgrBackground(color); !strings.Contains(frame, want) {
			t.Errorf("%v のフレームに背景色 %q が出てこない", kind, want)
		}
	}
}

func sgrBackground(color int) string {
	return esc + "48;5;" + strconv.Itoa(color) + "m"
}

// 背景色を解除し忘れると、塗った色が枠の外や次の行まで流れ出す。
// 解除は行末とは限らない（色が途切れたところで起きる）ので、
// 「最後に色を指定したあと、行が終わるまでに必ず解除がある」ことを見る。
func TestColorNeverLeaksPastTheEndOfALine(t *testing.T) {
	g := newGame(game.O)

	for i, line := range strings.Split(Frame(g), crlf) {
		last := strings.LastIndex(line, esc+"48;5;")
		if last < 0 {
			continue
		}
		if !strings.Contains(line[last:], resetGraphic) {
			t.Errorf("%d 行目が色を解除しないまま終わっている: %q", i, line)
		}
	}
}

// 差分描画をしていないので、フレームは引数だけで決まる。同じ状態からは何度呼んでも
// 同じ文字列が返る。これが崩れると、テストが「直前に何を描いたか」に依存し始める。
func TestFrameDependsOnlyOnTheGameState(t *testing.T) {
	g := newGame(game.T)

	first := Frame(g)
	second := Frame(g)

	if first != second {
		t.Error("同じ状態から 2 回描いて違うフレームが返った")
	}
}

// フレームは毎回カーソルを左上へ戻してから書く。画面消去はここではやらない（ちらつく）。
func TestFrameStartsByHomingTheCursor(t *testing.T) {
	frame := Frame(newGame(game.O))

	if !strings.HasPrefix(frame, cursorHome) {
		t.Errorf("フレームがカーソルを左上へ戻していない: %q", frame[:min(len(frame), 12)])
	}
	if strings.Contains(frame, clearScreen) {
		t.Error("フレームが画面消去を含んでいる（毎フレーム消すとちらつく）")
	}
}

// raw mode では LF だけだと行頭に戻らず、出力が階段状にずれる。
func TestEveryLineEndsWithCRLF(t *testing.T) {
	frame := Frame(newGame(game.O))

	lines := strings.Split(strings.TrimSuffix(frame, crlf), crlf)
	if len(lines) != game.Height+2 {
		t.Fatalf("CRLF で割ると %d 行になった（枠 2 行 + 盤面 %d 行のはず）", len(lines), game.Height)
	}
	if strings.Contains(strings.ReplaceAll(frame, crlf, ""), "\n") {
		t.Error("CRLF になっていない改行が混ざっている")
	}
}

func TestGameOverIsOverlaidOnTheBoard(t *testing.T) {
	g := newGameOver(t)

	lines := boardLines(t, Frame(g))
	row := lines[game.Height/2]

	if !strings.Contains(row, strings.TrimSpace(gameOverText)) {
		t.Fatalf("盤面の中央に %q が出ていない: %q", gameOverText, row)
	}

	// 左右の余りが同じくらいであること＝中央に寄っていること。
	left := strings.Index(row, "GAME")
	right := len(row) - strings.Index(row, "OVER") - len("OVER")
	if diff := left - right; diff > 2 || diff < -2 {
		t.Errorf("文字が中央に寄っていない（左 %d 文字、右 %d 文字）: %q", left, right, row)
	}
}

// ゲームオーバーでも積み上がった盤面は見えたままにする。
func TestGameOverKeepsTheBoardVisible(t *testing.T) {
	g := newGameOver(t)

	lines := boardLines(t, Frame(g))

	if !strings.ContainsRune(lines[len(lines)-1], '#') {
		t.Errorf("ゲームオーバー後に積み上がったセルが消えている: %q", lines[len(lines)-1])
	}
}

// ゲームオーバー後は操作対象のミノが無い（→ game.Active が false を返す）。
// それを描こうとして落ちないこと。
func TestGameOverFrameDoesNotDrawAnActiveMino(t *testing.T) {
	g := newGameOver(t)

	if _, ok := g.Active(); ok {
		t.Fatal("ゲームオーバーなのに操作対象のミノが残っている")
	}
	if frame := Frame(g); frame == "" {
		t.Error("ゲームオーバーのフレームが空になった")
	}
}

// Enter で隠したカーソルは Leave で必ず戻す。戻し忘れると、ゲームを抜けたあとの
// 端末でカーソルが見えないままになる。
func TestEnterAndLeaveArePaired(t *testing.T) {
	if !strings.Contains(Enter(), hideCursor) {
		t.Error("Enter がカーソルを隠していない")
	}
	if !strings.Contains(Leave(), showCursor) {
		t.Error("Leave がカーソルを戻していない")
	}
	if !strings.Contains(Enter(), clearScreen) {
		t.Error("Enter が画面を消していない")
	}
	if !strings.Contains(Leave(), resetGraphic) {
		t.Error("Leave が装飾を解除していない")
	}
}
