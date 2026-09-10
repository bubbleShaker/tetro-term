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

// 色を解除し忘れると、塗った色が枠や次の行へ流れ出す。
//
// 「解除のエスケープがどこかに出てくるか」を見るのでは足りない。解除が閉じ枠より
// **後ろ**にあっても文字列の中には存在してしまうし、絵の形も変わらないので、
// 形だけを見るテストはすり抜ける。枠そのものが何色で描かれたかを直接問う。
func TestBorderIsNeverPainted(t *testing.T) {
	// 塗られたセルが枠に接している状態で試す。**両端とも試すことに意味がある**。
	// 右端は「行の最後の文字が塗られたまま終わる」唯一の場合で、行末の解除を
	// 落とすとここだけが壊れる。左端で試しても、色は途中で切れるので気づけない。
	for _, tt := range []struct {
		name string
		move game.Input
	}{
		{"左端に寄せる", game.MoveLeft},
		{"右端に寄せる", game.MoveRight},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := newGame(game.O)
			for i := 0; i < game.Width; i++ {
				g.Handle(tt.move)
			}

			for i, p := range paintedRunes(Frame(g)) {
				if !strings.ContainsRune("+-|", p.ch) {
					continue
				}
				if p.st != plain {
					t.Fatalf("%d 文字目の枠 %q に装飾 %+v が乗っている（色が漏れている）",
						i, p.ch, p.st)
				}
			}
		})
	}
}

// 行をまたいで色が残っていないこと。
func TestColorDoesNotSurviveIntoTheNextLine(t *testing.T) {
	g := newGame(game.O)

	for i, p := range paintedRunes(Frame(g)) {
		if p.ch == '\n' && p.st != plain {
			t.Fatalf("%d 文字目の改行に装飾 %+v が残っている", i, p.st)
		}
	}
}

// フレームのバイト列そのものを固定する。
//
// ほかのテストは frameArt でエスケープを解釈し直してから検査しているが、それだと
// 「レンダラと読み戻しの両方が同じように間違っている」場合に気づけない。
// ここだけは 1 バイトも解釈せずに突き合わせる。
func TestFrameIsExactlyThisString(t *testing.T) {
	const (
		border   = "+--------------------+"
		emptyRow = "|                    |"
		// O ミノが出ている行。色を変える手前で必ず装飾を解除している。
		oRow = "|        " + resetGraphic + esc + "48;5;226m" + "    " + resetGraphic + "        |"
	)

	want := cursorHome +
		border + crlf +
		oRow + crlf +
		oRow + crlf +
		strings.Repeat(emptyRow+crlf, game.Height-2) +
		border

	if got := Frame(newGame(game.O)); got != want {
		t.Errorf("フレームが違う\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// 最終行に改行を付けると、端末の高さがちょうどフレームの高さのときに 1 行スクロールする。
// フレームはカーソルを左上へ戻して上書きするだけなので、一度ずれると直らない。
func TestFrameDoesNotEndWithANewline(t *testing.T) {
	frame := Frame(newGame(game.O))

	if strings.HasSuffix(frame, crlf) || strings.HasSuffix(frame, "\n") {
		t.Errorf("フレームが改行で終わっている: %q", frame[len(frame)-8:])
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
	if !strings.Contains(Enter(), resetGraphic) {
		t.Error("Enter が装飾を解除していない（起動前のシェルの色が残る）")
	}
	if !strings.Contains(Leave(), resetGraphic) {
		t.Error("Leave が装飾を解除していない")
	}

	// 最後のフレームには GAME OVER が出ている。出ぎわに消すと読む間もなく消える。
	if strings.Contains(Leave(), clearScreen) {
		t.Error("Leave が画面を消している（最後の画をプレイヤーが読めなくなる）")
	}
}

// Size が返す寸法は、ブラウザ版が端末を作るときの唯一の根拠になる。
// 実際のフレームを数えて突き合わせるので、盤面の大きさやセルの幅を変えたときに
// Size だけ古いまま残ることがない。
func TestSizeMatchesTheActualFrame(t *testing.T) {
	cols, rows := Size()

	lines := strings.Split(frameArt(Frame(newGame(game.O))), "\n")

	if len(lines) != rows {
		t.Errorf("Size は %d 行と言うが、フレームは %d 行ある", rows, len(lines))
	}
	for i, line := range lines {
		if got := len([]rune(line)); got != cols {
			t.Errorf("Size は %d 桁と言うが、%d 行目は %d 桁ある: %q", cols, i, got, line)
		}
	}
}
