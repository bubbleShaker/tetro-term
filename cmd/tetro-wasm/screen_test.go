package main

import "testing"

func TestFrameGateSendsTheFirstFrame(t *testing.T) {
	var s frameGate

	if !s.shouldSend("frame") {
		t.Error("最初のフレームが送られない。起動直後の画面が空のままになる")
	}
}

func TestFrameGateSkipsAnIdenticalFrame(t *testing.T) {
	var s frameGate
	s.shouldSend("frame")

	if s.shouldSend("frame") {
		t.Error("同じフレームを 2 度送っている")
	}
}

func TestFrameGateSendsAgainWhenTheFrameChanges(t *testing.T) {
	var s frameGate
	s.shouldSend("frame")
	s.shouldSend("frame")

	if !s.shouldSend("changed") {
		t.Error("絵が変わったのに送られない")
	}
	// 元に戻ったときも送らなければならない。覚えているのは「直前の 1 枚」であって、
	// 過去に送ったことのある絵の一覧ではない。
	if !s.shouldSend("frame") {
		t.Error("前に送ったことのある絵に戻ったとき送られない")
	}
}

// ゼロ値の frameGate が「空文字列を送り済み」と誤解しないこと。フレームが空になることは
// 実際には無いが、sent を消して frame == s.last だけで判断する実装に縮めると、
// この検査だけが落ちる。
func TestFrameGateSendsAnEmptyFirstFrame(t *testing.T) {
	var s frameGate

	if !s.shouldSend("") {
		t.Error("空のフレームが送り済みとして扱われている")
	}
}
