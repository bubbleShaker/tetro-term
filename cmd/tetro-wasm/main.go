//go:build js && wasm

// tetro-wasm はブラウザ側のエントリポイント。
//
// M0 の時点ではゲームは存在せず、「Go でビルドした WASM が JS 側の
// ターミナルへ文字を書ける」ことだけを確かめる。ここで通した経路が、
// 以降 ANSI フレーム文字列（docs/adr/0001）の通り道になる。
package main

import (
	"fmt"
	"runtime"
	"strings"
	"syscall/js"
)

// terminal は JS 側が用意したターミナル（web/main.js の globalThis.tetroTerm）への窓口。
// Go からブラウザの API を触る箇所をここに閉じ込めておくと、
// この先書くゲームのコアが syscall/js に汚染されずに済む。
type terminal struct {
	obj js.Value
}

func newTerminal() terminal {
	return terminal{obj: js.Global().Get("tetroTerm")}
}

// write は受け取った文字列をそのまま端末へ流す。
// 改行を CRLF に直すといった加工はここでは行わない。ADR-0001 のとおり
// 「フレームがコアと表示の唯一の境界」であり、改行の形もフレームを組み立てる
// 側の責務だからである（ここで変換すると、フレームが既に CRLF を含んでいた場合に
// 二重変換で壊れるうえ、CLI 版にも同じ変換を書く羽目になる）。
func (t terminal) write(s string) {
	t.obj.Call("write", s)
}

// ready は「Go の起動が終わった」ことを JS 側へ伝える。
func (t terminal) ready() {
	t.obj.Call("ready")
}

// 端末は raw mode 相当で動くため、行送りは CRLF でなければ行頭に戻らず、
// 出力が階段状にずれていく。
const crlf = "\r\n"

// bannerLines は "tetro-term" のアスキーアート。
var bannerLines = []string{
	`  _       _                _                   `,
	` | |_ ___| |_ _ __ ___ ___| |_ ___ _ __ _ __ ___ `,
	" | __/ _ \\ __| '__/ _ \\___| __/ _ \\ '__| '_ ` _ \\ ",
	` | ||  __/ |_| | | (_) |  | ||  __/ |  | | | | | |`,
	`  \__\___|\__|_|  \___/    \__\___|_|  |_| |_| |_|`,
}

// SGR は文字色を指定する ANSI エスケープ。ミノの色付けにも同じ仕組みを使うので、
// 経路が通っているかの確認を兼ねてここで一通り出しておく。
func sgr(code int, s string) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", code, s)
}

func banner() string {
	var sb strings.Builder
	for _, line := range bannerLines {
		sb.WriteString(sgr(36, line))
		sb.WriteString(crlf)
	}
	sb.WriteString(crlf)
	fmt.Fprintf(&sb, "  %s  (%s, %s/%s)%s",
		sgr(32, "Hello from Go"), runtime.Version(), runtime.GOOS, runtime.GOARCH, crlf)
	sb.WriteString(crlf)
	sb.WriteString("  " + sgr(90, "M0: Go → WASM → xterm.js → GitHub Pages の経路が通っている") + crlf)
	sb.WriteString("  " + sgr(90, "色と ANSI エスケープが解釈されていれば成功") + crlf)
	sb.WriteString(crlf)

	sb.WriteString("  ")
	for _, code := range []int{36, 33, 35, 32, 31, 34, 37} {
		sb.WriteString(sgr(code, "██") + " ")
	}
	sb.WriteString(crlf)

	return sb.String()
}

func main() {
	term := newTerminal()
	term.write(banner())
	term.ready()

	// WASM は main が return するとインスタンスごと終了し、以降 JS から
	// 呼び戻せなくなる。ゲームループを持つ M2 以降のために、ここで待機して
	// プロセスを生かしておく。
	<-make(chan struct{})
}
