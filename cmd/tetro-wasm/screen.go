// **このファイルにはビルドタグを付けない**（理由は clock.go の冒頭にある）。
package main

// frameGate は直前に送ったフレームを覚えておき、同じ絵を 2 度送らないようにする。
//
// 描画は毎フレーム（60 回/秒）呼ばれるが、実際に絵が変わるのはミノが動いたときだけで、
// 重力に至っては 800ms に 1 回しかない。同じ 1KB の文字列を秒 60 回 xterm.js へ
// 流し込む意味はない。
//
// これは render の「差分描画はしない」という方針と矛盾しない。レンダラは相変わらず
// 毎回まるごと組み立てる。**同じものを 2 度送らないのはドライバの判断**である。
type frameGate struct {
	last string
	sent bool
}

// shouldSend は今回組み立てたフレームを送るべきかを返し、送るなら覚え直す。
func (s *frameGate) shouldSend(frame string) bool {
	if s.sent && frame == s.last {
		return false
	}
	s.last = frame
	s.sent = true
	return true
}
