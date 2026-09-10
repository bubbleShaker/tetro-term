package game

import "time"

// Input はゲームに対して意味を持つ操作ひとつ（→ CONTEXT.md「入力」）。
//
// キーやボタンはここに現れない。矢印キーもタッチも、ドライバの側で入力へ
// 変換されてから渡ってくる。コアが知っているのは「左へ動かしたい」までである。
type Input uint8

const (
	MoveLeft Input = iota
	MoveRight
	SoftDrop
	RotateCW
	RotateCCW
)

// FallInterval は重力で 1 段落ちるまでの間隔。
// レベルに応じて縮めるのは M7（#8）の仕事で、いまは固定値。
const FallInterval = 800 * time.Millisecond

// Game は進行中の 1 ゲーム。盤面とアクティブミノ、そして落下の進み具合を持つ。
type Game struct {
	board   Board
	active  ActiveMino
	draw    func() MinoKind
	elapsed time.Duration
	over    bool
}

// New は新しいゲームを始める。
//
// draw は次に出すミノを 1 つ決める関数（→ CONTEXT.md「抽選」）。コアが math/rand を
// 直に呼ばずここを外から受け取るのは、テストで「I ミノが来たとき」を決め打ちできる
// ようにするためである。M5 の 7バッグも、コアではなくこの関数の差し替えとして入る。
func New(draw func() MinoKind) *Game {
	g := &Game{draw: draw}
	g.spawn()
	return g
}

// Update は経過時間の分だけゲームを進める。
//
// コアが時計を持たないのは、ブラウザ版と CLI 版で時間の刻み方が違ううえ、
// テストで実時間を待ちたくないためである（→ 0.5 秒進めたければ 0.5 秒を渡せばよい）。
func (g *Game) Update(dt time.Duration) {
	if g.over {
		return
	}
	g.elapsed += dt
	// dt が落下間隔を超えることもある（タブが背面にいた、描画が詰まった等）ので、
	// 1 段ではなく溜まった分だけ落とす。
	//
	// 途中でロックが起きると spawn が落下タイマーを 0 に戻すため、余っていた時間は
	// そこで捨てられ、このループも抜ける。新しく出てきたミノが、前のミノが溜めた時間の
	// せいでいきなり数段落ちる、という事故を防ぐためにそうしている。
	for g.elapsed >= FallInterval {
		g.elapsed -= FallInterval
		g.stepDown()
	}
}

// Handle は入力を 1 つ処理する。
func (g *Game) Handle(in Input) {
	if g.over {
		return
	}
	switch in {
	case MoveLeft:
		g.shift(-1)
	case MoveRight:
		g.shift(1)
	case SoftDrop:
		g.stepDown()
		// 落下タイマーを戻す。戻さないと、自分で落とした直後に重力の分がもう一段来て、
		// 一度の操作で 2 段落ちたように見えることがある。
		g.elapsed = 0
	case RotateCW:
		g.rotate(g.active.Rot.CW())
	case RotateCCW:
		g.rotate(g.active.Rot.CCW())
	}
}

// tryPlace は候補の位置へアクティブミノを移し、移せたかどうかを返す。
//
// 「いまの姿を複製し、動かし、そこに置けるか確かめ、置けるときだけ採用する」という手順は
// 落下にも横移動にも回転にも共通なので、ここに 1 つだけ置く。**盤面に触れずに候補を作れる**
// のは、アクティブミノが盤面の一部ではないから（→ CONTEXT.md「盤面」）である。
func (g *Game) tryPlace(candidate ActiveMino) bool {
	if g.board.collides(candidate) {
		return false
	}
	g.active = candidate
	return true
}

// stepDown は 1 段落とす。落とせなければ、そこでロックする。
//
// 「接地した瞬間」ではなく「接地したあと次に落とそうとした時」がロックの瞬間になる。
// つまり着地から最大で落下間隔ぶんだけ猶予が生まれる。M1 はこれを是とする
// （そのほうが遊びやすく、実装も素直なため）。#5 のロックディレイは、この偶然の猶予を
// 「動かすたびに測り直す 0.5 秒」という意図のあるルールへ置き換える作業になる。
func (g *Game) stepDown() {
	moved := g.active
	moved.Pos.Y++
	if !g.tryPlace(moved) {
		g.lockAndSpawn()
	}
}

func (g *Game) shift(dx int) {
	moved := g.active
	moved.Pos.X += dx
	g.tryPlace(moved)
}

// rotate は回転を試みる。
//
// ここにあるのは「候補のオフセットを順に試し、最初に成立したものを採る」という**形**だけで、
// 候補の中身は kicks が持つ**データ**である（→ docs/adr/0002）。M1 の候補は (0,0) ひとつなので
// 挙動は「その場で回せなければ何も起きない」に見えるが、M6 で表を差し替えると、
// この関数に一行も手を入れないまま壁際で正しく回るようになる。
func (g *Game) rotate(to Rotation) {
	for _, offset := range kicks(g.active.Kind, g.active.Rot, to) {
		candidate := g.active
		candidate.Rot = to
		candidate.Pos.X += offset.X
		candidate.Pos.Y += offset.Y
		if g.tryPlace(candidate) {
			return
		}
	}
}

func (g *Game) lockAndSpawn() {
	g.board.lock(g.active)
	g.board.clearLines()
	g.spawn()
}

// spawn は次のミノを抽選して出現させる。
// 出現位置に置けなければゲームオーバー（→ CONTEXT.md「盤面」に待避行が無いため）。
func (g *Game) spawn() {
	g.active = ActiveMino{Kind: g.draw(), Rot: Rot0, Pos: spawnPos}
	g.elapsed = 0
	if g.board.collides(g.active) {
		g.over = true
	}
}

// Board は盤面の**写し**を返す。
//
// 中身をそのまま渡すと、表示する側が盤面を書き換えられてしまう。「盤面を変えられるのは
// コアだけ」という約束をコメントではなく型で守るために複製を渡している。
// 200 セルの複製なので毎フレーム呼んでも問題にならない。
func (g *Game) Board() Board {
	return g.board
}

// Active はいま操作対象のミノを返す。ゲームオーバー後は操作対象が無いので false を返す。
func (g *Game) Active() (ActiveMino, bool) {
	if g.over {
		return ActiveMino{}, false
	}
	return g.active, true
}

// IsOver はゲームオーバーかどうかを返す。
func (g *Game) IsOver() bool {
	return g.over
}
