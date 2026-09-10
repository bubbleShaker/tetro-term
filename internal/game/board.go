package game

// 盤面の大きさ（→ CONTEXT.md「盤面」）。見えない待避行は設けていないので、
// 出現位置に置けないことがそのままゲームオーバーになる。
const (
	Width  = 10
	Height = 20
)

// Cell は盤面の 1 マス（→ CONTEXT.md「セル」）。
//
// 空であるか、どのミノ由来かだけを持つ。**色は持たない**——色は表示の都合であり、
// ミノの種類から色を決めるのは internal/render の仕事である（→ docs/adr/0001）。
type Cell struct {
	Filled bool
	Kind   MinoKind
}

// Board はロック済みのセルだけを持つグリッド（→ CONTEXT.md「盤面」）。
// アクティブミノは盤面の一部ではないので、ここには含まれない。
type Board struct {
	cells [Height][Width]Cell
}

// At は指定位置のセルを返す。盤面の外を指した場合は空のセルを返す。
func (b *Board) At(p Point) Cell {
	if !inside(p) {
		return Cell{}
	}
	return b.cells[p.Y][p.X]
}

func inside(p Point) bool {
	return p.X >= 0 && p.X < Width && p.Y >= 0 && p.Y < Height
}

// collides は、そのアクティブミノがいまその位置に存在できないかを返す。
// 盤面の外にはみ出しているか、ロック済みのセルと重なっていれば true。
func (b *Board) collides(m ActiveMino) bool {
	for _, p := range m.Cells() {
		if !inside(p) {
			return true
		}
		if b.cells[p.Y][p.X].Filled {
			return true
		}
	}
	return false
}

// lock はアクティブミノを盤面のセルとして確定させる（→ CONTEXT.md「ロック」）。
// 呼ぶ側が、そこに置けることを確かめてあること。
func (b *Board) lock(m ActiveMino) {
	for _, p := range m.Cells() {
		b.cells[p.Y][p.X] = Cell{Filled: true, Kind: m.Kind}
	}
}

// clearLines は埋まった行を取り除き、上の行を落とす（→ CONTEXT.md「ライン消去」）。
// 消した行数を返す。
//
// 下から上へ書き込み先 dst を進めていく方式にしてある。埋まった行に出会ったら
// dst を進めないので、その行は次に書き込まれる行に上書きされて消える。
// 「消してから上を全部ずらす」を行ごとに繰り返すより、盤面を 1 度なぞるだけで済む。
func (b *Board) clearLines() int {
	dst := Height - 1
	for src := Height - 1; src >= 0; src-- {
		if b.rowFilled(src) {
			continue
		}
		b.cells[dst] = b.cells[src]
		dst--
	}
	cleared := dst + 1
	// 残った上側は、下から詰め直した分だけ空になる。
	for row := dst; row >= 0; row-- {
		b.cells[row] = [Width]Cell{}
	}
	return cleared
}

func (b *Board) rowFilled(row int) bool {
	for x := 0; x < Width; x++ {
		if !b.cells[row][x].Filled {
			return false
		}
	}
	return true
}
