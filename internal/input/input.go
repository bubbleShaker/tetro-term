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
	escByte = 0x1b // ESC。矢印キーはこれで始まる並びとして届く
	ctrlC   = 0x03 // raw mode では Ctrl-C が SIGINT にならず、この 1 バイトが届く

	// 矢印キーの前置きには 2 通りある。ふだんは ESC [ だが、端末が
	// application cursor key mode に入っていると ESC O になる。どちらで来ても
	// 同じキーなので両方受ける。片方しか見ないと、設定次第で矢印が全部効かなくなる。
	csiBracket = '['
	ss3Letter  = 'O'
)

// maxSequence は 1 つの並びとして待つバイト数の上限。
//
// 終わりの来ない並びを延々と溜め込まないための歯止め。ここが無いと、壊れた
// 入力を渡されたときに持ち越しが際限なく膨らむ。
const maxSequence = 32

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

	if len(b) < 2 {
		return Action{}, 0, false // ESC だけ届いた。続きを待つ
	}

	switch b[1] {
	case csiBracket:
		return decodeCSI(b)
	case ss3Letter:
		if len(b) < 3 {
			return Action{}, 0, false
		}
		action, ok := arrow[b[2]]
		return action, 3, ok
	default:
		// ESC に別のキーが続いた（Alt + 何か など）。割り当てが無いので ESC だけ捨てる。
		// まとめて捨てないのは、続きのバイトが単体で意味を持つキーかもしれないため。
		return Action{}, 1, false
	}
}

// decodeCSI は ESC [ で始まる並びを 1 つ読む。
//
// この形の並びは「引数バイトと中間バイトが任意個続き、最後に終端バイトで終わる」と
// 決まっている。長さを 3 バイト決め打ちにすると、Ctrl+← のような修飾付きの並び
// （ESC [ 1 ; 5 D）で余りが出て、その残骸が別のキーとして読まれてしまう。
// 終端バイトまで数えて、まとめて 1 つとして扱う。
func decodeCSI(b []byte) (Action, int, bool) {
	for i := 2; i < len(b); i++ {
		switch c := b[i]; {
		case c >= 0x30 && c <= 0x3f, c >= 0x20 && c <= 0x2f:
			// 引数バイトと中間バイト。まだ続く。

		case c >= 0x40 && c <= 0x7e:
			// 終端バイト。ここで並びが終わる。
			// 矢印は「引数が何も付いていない」ものだけを受ける。修飾キー付きは
			// 割り当てが無いので、並び全体を読み飛ばす。
			if i == 2 {
				action, ok := arrow[c]
				return action, 3, ok
			}
			return Action{}, i + 1, false

		default:
			// 決まりから外れたバイト。ここまでを捨てて、そこから読み直す。
			return Action{}, i, false
		}
	}

	if len(b) >= maxSequence {
		// 終わりの見えない並び。ESC を 1 バイト捨てて読み直す。
		return Action{}, 1, false
	}
	return Action{}, 0, false // 終端がまだ届いていない
}
