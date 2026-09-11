package input

import "github.com/bubbleShaker/tetro-term/internal/game"

// タッチ操作のボタン列。
//
// **バイト列を経由しない**。矢印キーが ESC [ D として届くのは端末という入力装置の
// 都合であって、画面上のボタンにその都合は無い。ボタンのために JS がキーのバイト列を
// 組み立てる形にすると、「←は ESC [ D」という知識が Go の外へ漏れる——それは
// このパッケージが唯一の持ち主であるはずのものである。
//
// 代わりに、ここが「ボタンの一覧」と「id から入力への読み替え」の両方を持つ。
// JS が知るのは id と、そこに何と書いてあるかだけで、意味は知らない。
// KeyHelp() をここから配っているのと同じ考え方である。
//
// 入口は Decode と Press の 2 つになるが、どちらの出口も game.Input ひとつであり、
// 「入力がコア側から見た操作の唯一の境界」（→ CONTEXT.md「入力」）は保たれている。

// Side はボタンをどちらの親指に置くか。
//
// 縦持ちの画面幅いっぱいに 1 列で並べると、真ん中のボタンがどちらの親指からも
// 遠くなる。左右に分けて、それぞれの親指の届く範囲に寄せる。
//
// 数値ではなく文字列なのは、この値がそのまま JS へ渡るためである。
// 数えられる型にすると、向こう側に 0 が左だという対応表がもう 1 枚できてしまう。
type Side string

const (
	SideLeft  Side = "left"
	SideRight Side = "right"
)

// Button は画面に出すタッチボタン 1 つ。
//
// input が非公開なのは、ここを外から読めるようにすると Press を通らずに
// 入力を取り出せてしまうためである。読み替えの経路は 1 本に保つ。
type Button struct {
	ID    string // JS から押されたときに返ってくる名前
	Label string // ボタンに書く文字
	What  string // 読み上げ用の説明（aria-label）
	Side  Side
	// Repeat は押しっぱなしで繰り返すか。
	//
	// 繰り返しの間隔を決めるのはドライバ（web/main.js）だが、**どのボタンが
	// 繰り返してよいか**はここが決める。回転を繰り返させないのは、指を置いたまま
	// 回り続けると狙った向きで止められないからで、これはボタンの性格であって
	// 画面側の都合ではない。
	Repeat bool
	input  game.Input
}

// buttons は画面に出す順に並べる。
//
// 左は移動（←↓→ の並びは指の動きと向きが一致する）、右は回転。
// ハードドロップ（M4 / #5）とホールド（M5 / #6）はコアにまだ入力が無いので、
// ここにも無い。入力が増えたらこの表に 1 行足せば、ボタンも説明も一緒に増える——
// JS 側に足すものは何も無い。
var buttons = []Button{
	{ID: "left", Label: "←", What: "左へ移動", Side: SideLeft, Repeat: true, input: game.MoveLeft},
	{ID: "softDrop", Label: "↓", What: "ソフトドロップ", Side: SideLeft, Repeat: true, input: game.SoftDrop},
	{ID: "right", Label: "→", What: "右へ移動", Side: SideLeft, Repeat: true, input: game.MoveRight},
	{ID: "rotateCCW", Label: "↺", What: "反時計回りに回転", Side: SideRight, input: game.RotateCCW},
	{ID: "rotateCW", Label: "↻", What: "時計回りに回転", Side: SideRight, input: game.RotateCW},
}

// Buttons は画面に出すボタンの一覧を、並べる順に返す。
func Buttons() []Button {
	// 表そのものを渡すと、受け取った側が中身を書き換えられる。
	return append([]Button(nil), buttons...)
}

// Press は押されたボタンの id を入力に読み替える。
//
// 知らない id で false を返すのは、ここを通るのが自分たちの JS だけとはいえ、
// 綴りを間違えたときに黙って別の操作になるより、何も起きないほうが気づけるためである。
func Press(id string) (game.Input, bool) {
	for _, b := range buttons {
		if b.ID == id {
			return b.input, true
		}
	}
	return game.InputNone, false
}
