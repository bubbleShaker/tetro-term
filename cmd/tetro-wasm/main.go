//go:build js && wasm

// tetro-wasm はブラウザで遊ぶための入口。
//
// ここにゲームのルールは無い（→ CONTEXT.md「ドライバ」）。CLI 版（cmd/tetro）と
// 違うのは時間の測り方と画面の届け先だけで、コアもレンダラもキーの読み替えも
// 同じものを通る。だから両者で挙動が食い違わない。
//
// **ブラウザ版のドライバは JS と Go の 2 枚でひとつである**。時間を測って刻むのは
// web/main.js の requestAnimationFrame で、この Go 側は刻まれた時間を受け取って
// コアを進め、絵を返す。Go 側に time.Ticker を持たせていないのは、背面タブで
// setTimeout が 1 秒に 1 回まで絞られるためである（→ clock.go の maxDelta）。
package main

import (
	"syscall/js"
	"time"

	"github.com/bubbleShaker/tetro-term/internal/draw"
	"github.com/bubbleShaker/tetro-term/internal/game"
	"github.com/bubbleShaker/tetro-term/internal/input"
	"github.com/bubbleShaker/tetro-term/internal/render"
)

// terminal は JS 側が用意した窓口（web/main.js の globalThis.tetroTerm）。
//
// **ブラウザに触るのはこの型だけ**である。ここに閉じ込めておくことで、
// ゲームの進行を持つ driver は syscall/js を知らずに済む。
//
// 端末へ描くものはフレームだけだが（→ docs/adr/0001）、この型が持つのは
// それに限らない。ready と gameOver は端末の外の DOM——読み込み表示や
// ページ下の操作説明——への通知で、ADR が言う「唯一の境界」はあくまで
// 端末に描かれる絵についての約束である。
type terminal struct {
	obj js.Value
}

func newTerminal() terminal {
	return terminal{obj: js.Global().Get("tetroTerm")}
}

// write は受け取った文字列をそのまま端末へ流す。
// 改行を CRLF に直すといった加工はここでは行わない。ADR-0001 のとおり
// 「フレームがコアと表示の唯一の境界」であり、改行の形もフレームを組み立てる
// 側の責務だからである。
func (t terminal) write(s string) {
	t.obj.Call("write", s)
}

// ready は「Go の起動が終わった」ことを伝え、**同時に JS から Go を呼ぶ窓口を渡す**。
//
// 窓口をグローバルへ生やさず引数で渡しているのは、「準備ができた」と
// 「窓口が使える」が同じ 1 回の呼び出しで揃うからである。生やす方式だと
// JS 側は必ず「もう生えているか」を確かめる手順を持つことになり、
// その確認は書き忘れても大抵は動いてしまう類のコードになる。
//
// cols と rows を一緒に渡すのは、端末をフレームぴったりの大きさで作らせるため。
// 桁数を JS 側に書くと、盤面やセルの幅を変えたときに片方だけ古いまま残る。
func (t terminal) ready(cols, rows int, tick, data js.Func) {
	t.obj.Call("ready", map[string]any{
		"cols": cols,
		"rows": rows,
		"tick": tick,
		"data": data,
	})
}

// gameOver はゲームが終わったことを伝える。JS はページ下の案内を差し替える。
//
// M2 にリスタートは無いので（M7 / #8）、プレイヤーの行き先はリロードしかない。
// それを端末の中ではなく外に出しているのは、盤面には既に GAME OVER が
// 出ており、その上へ操作説明まで重ねると読めなくなるからである。
func (t terminal) gameOver() {
	t.obj.Call("gameOver")
}

// driver は 1 ゲーム分の進行を持つ。JS から呼ばれるのは tick と data の 2 つだけ。
type driver struct {
	term     terminal
	game     *game.Game
	decoder  input.Decoder
	screen   screen
	notified bool // gameOver を伝えたか
}

// tick は経過時間を受け取ってゲームを進め、画面を描き直す。
// dtMillis はミリ秒（JS の performance.now() の差）。
func (d *driver) tick(dtMillis float64) {
	d.game.Update(clampDelta(time.Duration(dtMillis * float64(time.Millisecond))))
	d.render()
}

// data は xterm.js が受け取ったバイト列を、そのまま入力として読み替える。
//
// 読み替えるのは CLI 版と同じ input.Decoder である。xterm.js は本物の端末と
// まったく同じバイト列を送ってくるので、キーの割り当てを 2 か所に持つ理由がない。
//
// **画面はここでは描かない**。次の tick が描くので、表示までは最大 16ms 遅れるが、
// これは人には知覚できない（CLI 版の刻みは 30ms なので、むしろ速い）。
// ここでも描くと、描画点が 2 か所になったうえ、キーリピート中は
// requestAnimationFrame より高い頻度で書き込みうる。
func (d *driver) data(bytes string) {
	for _, action := range d.decoder.Decode([]byte(bytes)) {
		// **終了はブラウザ版では何もしない**。q や Ctrl-C はプロセスを終える操作だが、
		// ブラウザにはプロセスの終わりが無い。「やめることはゲームのルールではない」以上
		// （→ input.Action）、それをどう解釈するかはドライバの自由であり、
		// ブラウザ版は「解釈しない」を選ぶ。
		if action.Quit {
			continue
		}
		d.game.Handle(action.Input)
	}
}

// render は絵が変わっていれば端末へ送り、ゲームが終わっていれば一度だけ知らせる。
func (d *driver) render() {
	if frame := render.Frame(d.game); d.screen.shouldSend(frame) {
		d.term.write(frame)
	}
	if d.game.IsOver() && !d.notified {
		d.notified = true
		d.term.gameOver()
	}
}

// callback は JS から呼ばれる関数を 1 つ作る。
//
// 引数が無い呼び出しを黙って無視するのは、ここで添字の範囲外に触れると
// panic が Go のインスタンスごと巻き込み、以降ゲームが二度と動かなくなるためである。
// 呼ぶのは自分たちの web/main.js だけとはいえ、代償が釣り合わない。
func callback(f func(js.Value)) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			f(args[0])
		}
		return nil
	})
}

func main() {
	d := &driver{term: newTerminal(), game: game.New(draw.Uniform())}

	// **描くより先に ready を呼ぶ**。JS はこの呼び出しの中で端末をフレームぴったりの
	// 大きさに作り替えるので、先に描くとその絵はリサイズで巻き込まれうる。そして
	// 描き直しは来ない——次の tick が組み立てるフレームは消える前とまったく同じ絵で、
	// screen が「送る必要なし」と判断してしまうからである。
	//
	// ready が戻ってから下の write までの間に requestAnimationFrame が割り込む心配は
	// 要らない。Go のインスタンスが JS へ制御を返すのは全ゴルーチンが止まったときだけで、
	// ここは何にも待たずに走り切る。
	//
	// js.FuncOf で作った関数は本来 Release が要るが、この 2 つはタブが閉じるまで
	// 呼ばれ続ける。解放するのは「もう呼ばれない」と決まったときで、その瞬間は来ない。
	cols, rows := render.Size()
	d.term.ready(cols, rows,
		callback(func(v js.Value) { d.tick(v.Float()) }),
		callback(func(v js.Value) { d.data(v.String()) }),
	)

	// 画面へ入る。CLI 版と違い、対になる render.Leave() は呼ばない——
	// ブラウザには「出ていく」瞬間が存在しないからである。カーソルを隠したまま
	// 終わることになるが、その端末はタブごと消える。
	d.term.write(render.Enter())
	// 最初の 1 枚をここで描いておく。最初の requestAnimationFrame までの
	// 数十 ms が、空っぽの端末にならないようにするため。
	d.render()

	// WASM は main が return するとインスタンスごと終了し、以降 JS から
	// 呼び戻せなくなる。tick と data を生かしておくために、ここで待ち続ける。
	<-make(chan struct{})
}
