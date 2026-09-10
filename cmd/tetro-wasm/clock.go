// **このファイルにはビルドタグを付けない。**
//
// 同じパッケージの main.go は js/wasm 専用なので、ホストの go test からは見えない。
// ブラウザに触らない判断——ここでは「時間をどう進めるか」——をこちらへ分けておくと、
// その部分だけは普通にテストできる。ここに syscall/js を持ち込んだ瞬間に、
// この性質は失われる（同じ理由で screen.go も分けてある）。
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
