// Package input は端末から届くバイト列を、ゲームへの入力に読み替える
// （→ CONTEXT.md「入力」「ドライバ」）。
//
// このパッケージが端末専用でないのは、xterm.js が本物の端末とまったく同じバイト列を
// 送ってくるからである。M2 でブラウザ版を書くとき、キーの読み替えはここを使い回せる。
// フレームが出口側の唯一の境界であるのと対称に（→ docs/adr/0001）、
// 入口側の境界も「バイト列ひとつ」に寄せてある。
package input

import "github.com/bubbleShaker/tetro-term/internal/game"

// Action はドライバが受け取る操作ひとつ。
//
// 終了がゲームへの入力ではなくここに混ざっているのは、**やめることはゲームのルールではない**
// からである。コアは盤面をどう変えるかしか知らず、プログラムを終える判断はドライバが持つ。
type Action struct {
	Input game.Input
	Quit  bool
}

// キーの割り当て。
//
// 押しっぱなしの左右移動を自前で作っていないのは、端末も xterm.js も
// 「キーを離した」を伝えてくれないため（→ PLAN.md の制約）。連続移動は
// OS のキーリピートに委ねる。
const (
	escByte    = 0x1b // ESC。矢印キーはこれで始まる 3 バイトの並びとして届く
	ctrlC      = 0x03 // raw mode では Ctrl-C が SIGINT にならず、この 1 バイトが届く
	csiBracket = '['  // ESC に続く「[」。ここまでが矢印キーの前置き
)

// singleByte は 1 バイトで決まるキー。
var singleByte = map[byte]Action{
	'q':   {Quit: true},
	'Q':   {Quit: true},
	ctrlC: {Quit: true},
	'z':   {Input: game.RotateCCW},
	'Z':   {Input: game.RotateCCW},
}

// arrow は ESC [ に続く 1 バイトで決まる矢印キー。
var arrow = map[byte]Action{
	'A': {Input: game.RotateCW},  // ↑
	'B': {Input: game.SoftDrop},  // ↓
	'C': {Input: game.MoveRight}, // →
	'D': {Input: game.MoveLeft},  // ←
}

// Decoder はバイト列を操作の列に読み替える。
//
// 1 回の読み取りが 1 回のキー入力に対応するとは限らないので、状態を持つ。
// キーリピートが効いていると複数のキーがまとめて届くし、逆に矢印キーの 3 バイトが
// 途中で切れて 2 回に分かれて届くこともある。どちらも取りこぼさないよう、
// 読み切れなかった端数を次回まで持ち越す。
type Decoder struct {
	pending []byte
}

// Decode は届いたバイト列を読み、決まった操作を順に返す。
// 読み切れなかった端数は中に残し、次の呼び出しで続きとして扱う。
func (d *Decoder) Decode(chunk []byte) []Action {
	d.pending = append(d.pending, chunk...)

	var actions []Action
	for len(d.pending) > 0 {
		action, n, ok := decodeOne(d.pending)
		if n == 0 {
			// 途中まで届いているだけ。続きが来るまで待つ。
			break
		}
		d.pending = d.pending[n:]
		if ok {
			actions = append(actions, action)
		}
	}
	if len(d.pending) == 0 {
		// 使い切った下地は捨てる。持ち越しが無いのに配列だけ育ち続けるのを防ぐ。
		d.pending = nil
	}
	return actions
}

// decodeOne は先頭のひとまとまりを読む。
// 読んだバイト数と、それが割り当てのあるキーだったかを返す。
// バイト数が 0 なら「まだ全部届いていない」という意味である。
func decodeOne(b []byte) (Action, int, bool) {
	if b[0] != escByte {
		action, ok := singleByte[b[0]]
		return action, 1, ok
	}

	// ここから矢印キーの可能性がある並び。
	if len(b) < 2 {
		return Action{}, 0, false // ESC だけ届いた。続きを待つ
	}
	if b[1] != csiBracket {
		// ESC に別のキーが続いた（Alt + 何か など）。割り当てが無いので ESC だけ捨てる。
		// まとめて捨てないのは、続きのバイトが単体で意味を持つキーかもしれないため。
		return Action{}, 1, false
	}
	if len(b) < 3 {
		return Action{}, 0, false // ESC [ まで届いた。続きを待つ
	}

	action, ok := arrow[b[2]]
	return action, 3, ok
}
