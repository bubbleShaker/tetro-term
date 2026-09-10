// Package draw は次のアクティブミノとして何を出すかを決める（→ CONTEXT.md「抽選」）。
//
// コアの外にあるのは、コアが「次の 1 つ」を求めるだけで、それが完全ランダムなのか
// 7バッグなのかを知らないからである（→ docs/adr/0001 の「コアは環境を知らない」）。
// 実際 internal/game は math/rand を import できない——layering の決まりが禁じている。
//
// **入口ごとに抽選を書かない**ために、このパッケージがある。CLI 版とブラウザ版が
// 別々の抽選を持っていると、M5（#6）で 7バッグへ差し替えたとき片方だけ古いまま残りうる。
// しかもその時ブラウザ版でしか遊んでいなければ、誰も気づかない。
package draw

import (
	"math/rand/v2"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

// Uniform は 7 種から一様な確率で 1 つ選ぶ抽選を返す。
//
// 同じミノが何度も続いて理不尽になるのは承知のうえで、まずこれを使う。
// それを直すのが M5（#6）の 7バッグで、そのときは game.New へ渡すものが
// この関数から別の関数に変わるだけで、コアには手が入らない。
func Uniform() func() game.MinoKind {
	kinds := game.AllKinds()
	return func() game.MinoKind {
		return kinds[rand.IntN(len(kinds))]
	}
}
