// **このファイルにはビルドタグを付けない。**
//
// 同じパッケージの main.go は js/wasm 専用なので、ホストの go test からは見えない。
// ブラウザに触らない判断——時間をどう進めるか、いつ画面を送るか——をこちらへ分けておくと、
// その部分だけは普通にテストできる。ここに syscall/js を持ち込んだ瞬間に、
// この性質は失われる。
//
// ホストで go build ./... だけは「func main が無い」と言って落ちるが、
// CI が回すのはテストと js/wasm 向けの vet とブラウザ版のビルドだけなので支障はない。
package main

import "time"

// maxDelta は 1 回の tick で受け付ける経過時間の上限。
//
// ブラウザのタブが背面に回ると requestAnimationFrame は止まり、戻ってきたときの
// 経過時間は数秒から数分になりうる。それをそのままコアへ渡すと、コアは
// 「溜まった分だけ落とす」ので（→ game.Update）、画面を見た瞬間にミノが
// 一気に何段も落ちて、置きたかった場所を通り過ぎている。
//
// 100ms は落下間隔 800ms の 1/8 で、1 回の Update で落ちるのは高々 1 段になる。
// 描画は 16ms 間隔なので、6 フレームぶんの遅れまでは正直に反映される計算になる。
// 上限を超えた分は捨てる——見ていない間は時間が進まない、というのが意図した挙動である。
const maxDelta = 100 * time.Millisecond

// clampDelta は経過時間を扱える範囲に収める。
//
// 負の値も潰している。requestAnimationFrame に渡される時刻は単調増加すると
// 決められているものの、ここが負のまま通ると elapsed が巻き戻って
// ミノが落ちなくなる。原因の分かりにくい固まり方をするので、入口で止める。
func clampDelta(d time.Duration) time.Duration {
	switch {
	case d < 0:
		return 0
	case d > maxDelta:
		return maxDelta
	default:
		return d
	}
}

// screen は直前に送ったフレームを覚えておき、同じ絵を 2 度送らないようにする。
//
// 描画は毎フレーム（60 回/秒）呼ばれるが、実際に絵が変わるのはミノが動いたときだけで、
// 重力に至っては 800ms に 1 回しかない。同じ 1KB の文字列を秒 60 回 xterm.js へ
// 流し込む意味はない。
//
// これは render の「差分描画はしない」という方針と矛盾しない。レンダラは相変わらず
// 毎回まるごと組み立てる。**同じものを 2 度送らないのはドライバの判断**である。
type screen struct {
	last string
	sent bool
}

// shouldSend は今回組み立てたフレームを送るべきかを返し、送るなら覚え直す。
func (s *screen) shouldSend(frame string) bool {
	if s.sent && frame == s.last {
		return false
	}
	s.last = frame
	s.sent = true
	return true
}
