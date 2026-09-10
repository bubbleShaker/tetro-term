// Package game は落ち物パズルのルールそのものを持つ。
//
// このパッケージは端末もブラウザも時計も知らない（→ docs/adr/0001）。
// 時間は外から注入され、操作は入力として渡され、結果は盤面の状態として返る。
// import に環境依存のものが現れたら、それは設計が崩れた合図である。
package game

import "fmt"

// Point は盤面上の位置。X は列（右が正）、Y は行（**下が正**）。
// 画面の座標系にそろえてあるので、Y が増えることがそのまま落下になる。
type Point struct {
	X, Y int
}

// MinoKind はミノの種類（→ CONTEXT.md「ミノ」）。
type MinoKind uint8

const (
	I MinoKind = iota
	O
	T
	S
	Z
	J
	L
)

// Kinds は 7 種すべて。抽選（→ CONTEXT.md）の実装が母集団として使う。
var Kinds = [7]MinoKind{I, O, T, S, Z, J, L}

func (k MinoKind) String() string {
	// 添字は上の const の並びと対応している。
	return [...]string{"I", "O", "T", "S", "Z", "J", "L"}[k]
}

// Rotation は回転状態（→ CONTEXT.md「回転状態」）。角度ではなく 4 つの離散した向き。
type Rotation uint8

const (
	Rot0 Rotation = iota
	RotR
	Rot2
	RotL
)

func (r Rotation) String() string {
	return [...]string{"0", "R", "2", "L"}[r]
}

// CW は時計回りに 1 つ進めた回転状態を返す。
func (r Rotation) CW() Rotation { return (r + 1) % 4 }

// CCW は反時計回りに 1 つ進めた回転状態を返す。
// -1 ではなく +3 なのは、Rotation が符号なしのため 0 から引くと巨大な値へ回り込むからである。
func (r Rotation) CCW() Rotation { return (r + 3) % 4 }

// boxSize はミノの形を収める正方形の一辺。7 種すべてを同じ大きさの箱に入れておくと、
// 出現位置の計算も衝突判定も種類ごとに分岐せずに済む。
const boxSize = 4

// shape は 1 つの形。箱の左上を原点とした 4 セルの相対位置。
type shape [4]Point

// shapeArt は 7 種 × 4 回転状態 = 28 個の形を、そのまま目で読める絵として書き下ろしたもの。
//
// 実行時に座標を回して求めていないのは、回転が「90 度回す」で説明しきれないためである。
// O は回しても動いてはならず、I は回転の中心が格子の交点にあってセルの上に乗らない。
// さらに M6 で入れる SRS のキックテーブル（→ docs/adr/0002）は、ここに書き下ろした形を
// 前提にオフセットが決められている。計算で求めた形とは噛み合わない。
//
// 並びは Rot0, RotR, Rot2, RotL の順。'X' が埋まっているセル、それ以外は空白として無視される。
var shapeArt = map[MinoKind][4]string{
	I: {`
		....
		XXXX
		....
		....`, `
		..X.
		..X.
		..X.
		..X.`, `
		....
		....
		XXXX
		....`, `
		.X..
		.X..
		.X..
		.X..`},

	// O だけは 4 つの回転状態が同じ絵になる。「回しても動かない」をここで表現している。
	O: {`
		.XX.
		.XX.
		....
		....`, `
		.XX.
		.XX.
		....
		....`, `
		.XX.
		.XX.
		....
		....`, `
		.XX.
		.XX.
		....
		....`},

	T: {`
		.X..
		XXX.
		....
		....`, `
		.X..
		.XX.
		.X..
		....`, `
		....
		XXX.
		.X..
		....`, `
		.X..
		XX..
		.X..
		....`},

	S: {`
		.XX.
		XX..
		....
		....`, `
		.X..
		.XX.
		..X.
		....`, `
		....
		.XX.
		XX..
		....`, `
		X...
		XX..
		.X..
		....`},

	Z: {`
		XX..
		.XX.
		....
		....`, `
		..X.
		.XX.
		.X..
		....`, `
		....
		XX..
		.XX.
		....`, `
		.X..
		XX..
		X...
		....`},

	J: {`
		X...
		XXX.
		....
		....`, `
		.XX.
		.X..
		.X..
		....`, `
		....
		XXX.
		..X.
		....`, `
		.X..
		.X..
		XX..
		....`},

	L: {`
		..X.
		XXX.
		....
		....`, `
		.X..
		.X..
		.XX.
		....`, `
		....
		XXX.
		X...
		....`, `
		XX..
		.X..
		.X..
		....`},
}

// shapes は shapeArt を座標に直したもの。当たり判定は絵ではなく座標で行う。
var shapes = map[MinoKind][4]shape{}

// init は起動時に絵を座標へ変換する。
//
// 絵が壊れていたら panic する。これは実行時の異常ではなく、上の表を書き間違えたという
// プログラマの誤りであり、テストを 1 つでも動かせば必ず起動時に露見する。
func init() {
	for kind, arts := range shapeArt {
		var parsed [4]shape
		for rot, art := range arts {
			parsed[rot] = parseShape(kind, Rotation(rot), art)
		}
		shapes[kind] = parsed
	}
}

func parseShape(kind MinoKind, rot Rotation, art string) shape {
	var s shape
	filled, seen := 0, 0
	for _, r := range art {
		if r != 'X' && r != '.' {
			// 改行やインデントは絵を読みやすくするためだけのもの。
			continue
		}
		if r == 'X' {
			if filled == len(s) {
				panic(fmt.Sprintf("game: %v %v の形にセルが %d 個以上ある", kind, rot, len(s)+1))
			}
			s[filled] = Point{X: seen % boxSize, Y: seen / boxSize}
			filled++
		}
		seen++
	}
	if seen != boxSize*boxSize {
		panic(fmt.Sprintf("game: %v %v の形が %dx%d でなく %d セルある",
			kind, rot, boxSize, boxSize, seen))
	}
	if filled != len(s) {
		panic(fmt.Sprintf("game: %v %v の形のセルが %d 個しかない", kind, rot, filled))
	}
	return s
}

// ActiveMino はいま操作対象になっているミノ（→ CONTEXT.md「アクティブミノ」）。
//
// Pos が指すのはミノそのものの位置ではなく、**形を収める箱の左上**が盤面のどこにあるか。
// 形の側が箱の中での位置を持っているので、こうしておくと移動も回転も箱をずらすだけになる。
type ActiveMino struct {
	Kind MinoKind
	Rot  Rotation
	Pos  Point
}

// Cells はアクティブミノが占める 4 セルの、盤面上の絶対位置を返す。
func (m ActiveMino) Cells() [4]Point {
	var cells [4]Point
	for i, p := range shapes[m.Kind][m.Rot] {
		cells[i] = Point{X: m.Pos.X + p.X, Y: m.Pos.Y + p.Y}
	}
	return cells
}

// spawnPos は新しいミノが出現するときの箱の左上。
// X=3 に置くと、3 セル幅のミノは 3〜5 列目、I ミノは 3〜6 列目に出る（一般的な配置）。
var spawnPos = Point{X: 3, Y: 0}
