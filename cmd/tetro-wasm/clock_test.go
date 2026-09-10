package main

import (
	"testing"
	"time"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

func TestClampDeltaPassesOrdinaryFramesThrough(t *testing.T) {
	// 60fps の 1 フレーム。ふだん渡ってくるのはこの辺りの値で、
	// ここに手が入ると操作の手触りが変わってしまう。
	const oneFrame = 17 * time.Millisecond

	if got := clampDelta(oneFrame); got != oneFrame {
		t.Errorf("%v がそのまま通るべきだが %v になった", oneFrame, got)
	}
}

func TestClampDeltaCutsOffTheGapAfterTheTabComesBack(t *testing.T) {
	if got := clampDelta(30 * time.Second); got != maxDelta {
		t.Errorf("背面タブから戻ったときの %v は %v に切り詰められるべきだが %v になった",
			30*time.Second, maxDelta, got)
	}
}

func TestClampDeltaRefusesToGoBackwards(t *testing.T) {
	if got := clampDelta(-1 * time.Second); got != 0 {
		t.Errorf("負の経過時間は 0 に潰されるべきだが %v になった", got)
	}
}

// 切り詰めた経過時間をコアへ渡しても、1 回で 2 段以上落ちないこと。
// maxDelta を落下間隔より大きい値に書き換えてしまうと、背面タブから戻った瞬間に
// ミノが飛ぶ。定数どうしの大小を直に比べるのではなく、コアに実際に渡して
// **落ちた段数**で確かめる——守りたいのは定数の関係ではなく、この結果のほうである。
func TestClampedDeltaNeverDropsTheMinoMoreThanOneRow(t *testing.T) {
	g := game.New(func() game.MinoKind { return game.O })

	before, ok := g.Active()
	if !ok {
		t.Fatal("始まった直後にアクティブミノが無い")
	}

	g.Update(clampDelta(30 * time.Second))

	after, ok := g.Active()
	if !ok {
		t.Fatal("1 回進めただけでゲームオーバーになった")
	}
	if dropped := after.Pos.Y - before.Pos.Y; dropped > 1 {
		t.Errorf("1 回の tick で %d 段落ちた。切り詰めが効いていない", dropped)
	}
}
