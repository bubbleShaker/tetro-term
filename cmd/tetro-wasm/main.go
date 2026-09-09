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

// bridge は JS 側が用意したオブジェクト（web/main.js の globalThis.tetroTerm）。
// Go からブラウザの API を触る唯一の窓口をここに集約しておくと、
// コアが syscall/js に汚染されずに済む。
type bridge struct {
	obj js.Value
}

func newBridge() bridge {
	return bridge{obj: js.Global().Get("tetroTerm")}
}

// write はターミナルへ文字列を書く。改行は端末の作法に合わせて CRLF にする
// （raw mode の端末では "\n" だけだと行頭に戻らず、階段状にずれていく）。
func (b bridge) write(s string) {
	b.obj.Call("write", strings.ReplaceAll(s, "\n", "\r\n"))
}

const banner = "" +
	"\x1b[36m" + `  _       _                _                   ` + "\x1b[0m\n" +
	"\x1b[36m" + ` | |_ ___| |_ _ __ ___ ___| |_ ___ _ __ _ __ ___ ` + "\x1b[0m\n" +
	"\x1b[36m" + ` | __/ _ \ __| '__/ _ \___| __/ _ \ '__| '_ \` + "` _ \\" + ` ` + "\x1b[0m\n" +
	"\x1b[36m" + ` | ||  __/ |_| | | (_) |  | ||  __/ |  | | | | | |` + "\x1b[0m\n" +
	"\x1b[36m" + `  \__\___|\__|_|  \___/    \__\___|_|  |_| |_| |_|` + "\x1b[0m\n"

func main() {
	term := newBridge()

	term.write(banner)
	term.write("\n")
	term.write(fmt.Sprintf("  \x1b[32mHello from Go\x1b[0m  (%s, %s/%s)\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH))
	term.write("\n")
	term.write("  \x1b[90mM0: Go → WASM → xterm.js → GitHub Pages の経路が通っている\x1b[0m\n")
	term.write("  \x1b[90m色と ANSI エスケープが解釈されていれば成功\x1b[0m\n")

	// 生存確認用に、色付きのブロックを1行描く。
	// ミノの表示に同じ手法を使うので、色が出るかをここで確かめておく。
	var sb strings.Builder
	sb.WriteString("\n  ")
	for _, c := range []int{36, 33, 35, 32, 31, 34, 37} {
		fmt.Fprintf(&sb, "\x1b[%dm██\x1b[0m ", c)
	}
	sb.WriteString("\n")
	term.write(sb.String())

	term.obj.Call("ready")

	// WASM は main が return するとインスタンスごと終了し、以降 JS から
	// 呼び戻せなくなる。ゲームループを持つ M2 以降のために、ここで待機して
	// プロセスを生かしておく。
	<-make(chan struct{})
}
