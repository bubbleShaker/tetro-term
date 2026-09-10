package draw

import (
	"testing"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

// drawCount は確率的な検査で引く回数。
//
// 7 種のどれかが 1 度も出ない確率は 7×(6/7)^700 で、10 の 45 乗分の 1 より小さい。
// この検査が気まぐれに落ちることは、実際には起こらないと考えてよい。
const drawCount = 700

// 添字の計算を 1 つ間違えるだけで、特定のミノが永久に出なくなったり、
// 範囲外で落ちたりする。どちらも遊んでいる最中には気づきにくいので機械に見張らせる。
func TestUniformEventuallyReturnsEveryKind(t *testing.T) {
	next := Uniform()

	seen := map[game.MinoKind]int{}
	for i := 0; i < drawCount; i++ {
		seen[next()]++
	}

	for _, kind := range game.AllKinds() {
		if seen[kind] == 0 {
			t.Errorf("%d 回引いて %v が 1 度も出なかった: %v", drawCount, kind, seen)
		}
	}
}

// 母集団の外の値が出ないこと。MinoKind は uint8 なので、範囲を外れた添字が
// そのまま別の種類として通り、レンダラが色を引けずに落ちる、という形で表面化する。
func TestUniformReturnsOnlyKnownKinds(t *testing.T) {
	known := map[game.MinoKind]bool{}
	for _, kind := range game.AllKinds() {
		known[kind] = true
	}

	next := Uniform()
	for i := 0; i < drawCount; i++ {
		if got := next(); !known[got] {
			t.Fatalf("7 種にない値が出た: %v", got)
		}
	}
}

// 抽選は呼ぶたびに独立していなければならない。同じ値を返し続ける実装
// （たとえば乱数を関数の外で 1 度だけ引いてしまった場合）をここで捕まえる。
func TestUniformDoesNotReturnTheSameKindForever(t *testing.T) {
	next := Uniform()

	first := next()
	for i := 0; i < drawCount; i++ {
		if next() != first {
			return
		}
	}
	t.Fatalf("%d 回引いて %v しか出なかった", drawCount, first)
}
