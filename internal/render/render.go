// Package render は盤面をフレーム——ANSI エスケープを含んだ 1 描画分の文字列——に直す
// （→ CONTEXT.md「フレーム」）。
//
// ANSI エスケープを組み立てるのはこのパッケージだけである。コアは色も枠も知らず、
// ドライバはここが返した文字列をそのまま流すだけでよい（→ docs/adr/0001）。
// CLI 版とブラウザ版で見た目が食い違わないのは、両者が同じ文字列を受け取るからである。
package render

import (
	"fmt"
	"strings"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

// cellWidth は 1 セルを何文字で描くか。
//
// 半角 2 文字にしているのは、盤面の見た目を正方形に近づけるため。文字は「█」ではなく
// **空白に背景色を塗る**。「█」や罫線素片は端末によって全角として扱われ、
// そうなると盤面の幅が倍になって崩れる。空白の幅にはどの端末でも解釈の揺れがない。
const cellWidth = 2

// 端末は raw mode で動くため、行送りは CRLF でなければ行頭に戻らず、出力が階段状にずれる。
const crlf = "\r\n"

// ANSI エスケープ。
const (
	esc          = "\x1b["
	cursorHome   = esc + "H"    // カーソルを左上へ
	clearScreen  = esc + "2J"   // 画面全体を消す
	hideCursor   = esc + "?25l" // カーソルを隠す
	showCursor   = esc + "?25h" // カーソルを戻す
	resetGraphic = esc + "0m"   // 色などの装飾を解除する
)

// noColor は「塗らない」を表す。端末が元々持っている背景色がそのまま見える。
const noColor = -1

// minoColor はミノの種類ごとの色（256 色モードの色番号）。
//
// **色を決めるのはここだけ**である。コアのセルが持っているのは種別だけで、
// 色は表示の都合だから（→ docs/adr/0001）。
//
// 8 色モードではなく 256 色モードを使っているのは、L ミノの橙色が 8 色の中に無いため。
// 橙を諦めて他の色と重複させると、盤面の上で L と他のミノが見分けられなくなる。
var minoColor = map[game.MinoKind]int{
	game.I: 51,  // 水色
	game.O: 226, // 黄
	game.T: 201, // 紫
	game.S: 46,  // 緑
	game.Z: 196, // 赤
	game.J: 21,  // 青
	game.L: 208, // 橙
}

// 盤面へ重ねる文字の色。ミノの色と同じく、決めるのはこのファイルの中だけ。
// 下に何色のミノが積まれていても読めるよう、文字自身が背景ごと塗り替える。
const (
	overlayFG = 231 // ほぼ白
	overlayBG = 16  // ほぼ黒
)

// gameOverText はゲームオーバー時に盤面へ重ねる文字。前後の空白は、
// 下にあるミノと文字がくっついて読みにくくなるのを避けるためのもの。
const gameOverText = " GAME OVER "

func init() {
	// 表に載せ忘れた種類があると、そのミノだけ色を失って盤面から消える。
	// 起動した時点で気づけるようにここで確かめる。
	for _, kind := range game.AllKinds() {
		if _, ok := minoColor[kind]; !ok {
			panic(fmt.Sprintf("render: %v の色が決まっていない", kind))
		}
	}
	// 盤面より長い文字は重ねられない。重ねようとすると盤面の外を書きに行って落ちる。
	if n := len([]rune(gameOverText)); n > boardWidth {
		panic(fmt.Sprintf("render: 重ねる文字が %d 文字あり、盤面の幅 %d に収まらない", n, boardWidth))
	}
}

// style は 1 文字ぶんの装飾。
type style struct {
	fg, bg int
}

var plain = style{fg: noColor, bg: noColor}

// glyph は画面に置かれる 1 文字と、その装飾。
//
// 文字列を継ぎ足しながら組み立てるのではなく、いったん「文字の格子」を作ってから
// 文字列にしている。エスケープが混ざった文字列は添字で切り貼りできないため、
// GAME OVER のような**重ね書き**が文字列のままでは書けないからである。
type glyph struct {
	ch rune
	st style
}

// boardWidth は盤面が占める文字数。
const boardWidth = game.Width * cellWidth

// Enter は画面に入るときに一度だけ流す文字列を返す。
//
// フレームはカーソルを左上へ戻して上書きするだけなので、画面を消すのはここで一度きり。
// 毎フレーム消すとちらつく。
//
// 先に装飾を解除するのは、ゲームを起動した時点のシェルに色が残っていることがあるため。
// 解除しないと、レンダラが何も塗っていない枠や空きマスがその色のまま出続ける。
func Enter() string {
	return resetGraphic + clearScreen + hideCursor + cursorHome
}

// Leave は画面から出るときに流す文字列を返す。
//
// Enter で隠したカーソルを必ず戻す。戻し忘れると、ゲームを終えたあとの端末で
// カーソルが見えないままになる。Enter とは必ず対で呼ぶこと。
//
// **画面は消さない**。最後のフレームには GAME OVER が出ているのに、出ぎわに消すと
// プレイヤーはそれを読む間もなくシェルに戻される。最後の画を残したまま、
// その下から続きを始められるように改行だけ足す。
func Leave() string {
	return resetGraphic + showCursor + crlf
}

// Frame は 1 描画分のフレームを返す（→ CONTEXT.md「フレーム」）。
//
// 変わったところだけを塗り直す差分描画はしていない。毎回かならず 1 画面ぶんすべてを
// 書くので、この関数は直前に何を描いたかを覚えている必要がない。おかげで戻り値は
// 引数だけで決まり、テストが文字列の比較だけで書ける。200 セルの塗り直しは端末にとって
// 何の負担でもないので、差分にする理由がない。
func Frame(g *game.Game) string {
	cells := layout(g)

	var sb strings.Builder
	sb.WriteString(cursorHome)

	border := "+" + strings.Repeat("-", boardWidth) + "+"
	sb.WriteString(border + crlf)
	for _, row := range cells {
		sb.WriteString("|")
		writeRow(&sb, row[:])
		sb.WriteString("|" + crlf)
	}
	// 最後の行には改行を付けない。付けるとカーソルが 1 行下へ進み、端末の高さが
	// ちょうどフレームの高さだったときに画面が 1 行ぶんスクロールしてしまう。
	// フレームはカーソルを左上へ戻して上書きするだけなので、一度ずれると
	// 以降ずっとずれたまま描き続けることになる。
	sb.WriteString(border)

	return sb.String()
}

// layout は盤面・アクティブミノ・ゲームオーバー表示を、この順に重ねた文字の格子を作る。
// 順序に意味がある。あとに重ねたものが上に見える。
func layout(g *game.Game) [game.Height][boardWidth]glyph {
	var cells [game.Height][boardWidth]glyph
	for y := range cells {
		for x := range cells[y] {
			cells[y][x] = glyph{ch: ' ', st: plain}
		}
	}

	board := g.Board()
	for y := 0; y < game.Height; y++ {
		for x := 0; x < game.Width; x++ {
			if cell := board.At(game.Point{X: x, Y: y}); cell.Filled {
				paintCell(&cells, x, y, cell.Kind)
			}
		}
	}

	// アクティブミノは盤面の一部ではないので（→ CONTEXT.md「盤面」）、別に重ねる。
	if active, ok := g.Active(); ok {
		for _, p := range active.Cells() {
			paintCell(&cells, p.X, p.Y, active.Kind)
		}
	}

	if g.IsOver() {
		overlayText(&cells, gameOverText)
	}
	return cells
}

// paintCell は盤面の 1 マスを、cellWidth ぶんの文字として塗る。
func paintCell(cells *[game.Height][boardWidth]glyph, x, y int, kind game.MinoKind) {
	st := style{fg: noColor, bg: minoColor[kind]}
	for i := 0; i < cellWidth; i++ {
		cells[y][x*cellWidth+i] = glyph{ch: ' ', st: st}
	}
}

// overlayText は盤面の中央へ文字を重ねる。
func overlayText(cells *[game.Height][boardWidth]glyph, text string) {
	runes := []rune(text)
	left := (boardWidth - len(runes)) / 2
	row := game.Height / 2
	// 文字は下にあるミノの色を受け継がず、自前の色で塗る。そうしないと、
	// 積み上がり方によって読めたり読めなかったりする。
	st := style{fg: overlayFG, bg: overlayBG}
	for i, r := range runes {
		cells[row][left+i] = glyph{ch: r, st: st}
	}
}

// writeRow は 1 行ぶんの文字を、装飾が変わるところでだけエスケープを挟みながら書く。
// 1 文字ごとに色を指定し直すと、見た目は同じでもフレームが数倍に膨らむ。
func writeRow(sb *strings.Builder, row []glyph) {
	current := plain
	for _, gl := range row {
		if gl.st != current {
			sb.WriteString(sgr(gl.st))
			current = gl.st
		}
		sb.WriteRune(gl.ch)
	}
	// 行の終わりで必ず装飾を解除する。解除し忘れると、背景色が枠や次の行へ流れ出す。
	if current != plain {
		sb.WriteString(resetGraphic)
	}
}

// sgr は装飾を指定する ANSI エスケープを返す。
// SGR は Select Graphic Rendition の略で、色や太字を選ぶための命令のこと。
func sgr(st style) string {
	if st == plain {
		return resetGraphic
	}
	var sb strings.Builder
	// 前の装飾が残らないよう、いったん解除してから指定する。
	sb.WriteString(resetGraphic)
	if st.fg != noColor {
		fmt.Fprintf(&sb, "%s38;5;%dm", esc, st.fg)
	}
	if st.bg != noColor {
		fmt.Fprintf(&sb, "%s48;5;%dm", esc, st.bg)
	}
	return sb.String()
}
