package game

import (
	"testing"
	"time"
)

func TestGravityDropsOneStepPerInterval(t *testing.T) {
	g := New(fixedDraw(O))
	before, _ := g.Active()

	g.Update(FallInterval)

	after, _ := g.Active()
	if got, want := after.Pos.Y, before.Pos.Y+1; got != want {
		t.Errorf("落下後の行が %d、期待は %d", got, want)
	}
}

func TestGravityDoesNotDropBeforeTheIntervalElapses(t *testing.T) {
	g := New(fixedDraw(O))
	before, _ := g.Active()

	// 半分ずつ 2 回で、ちょうど 1 段。時間が足し合わされていることを見ている。
	g.Update(FallInterval / 2)
	if mid, _ := g.Active(); mid.Pos.Y != before.Pos.Y {
		t.Fatalf("間隔の半分で落ちてしまった: %d", mid.Pos.Y)
	}
	g.Update(FallInterval / 2)
	if after, _ := g.Active(); after.Pos.Y != before.Pos.Y+1 {
		t.Errorf("間隔ぶん経っても落ちていない: %d", after.Pos.Y)
	}
}

// 描画が詰まったりタブが背面に回ったりして dt が大きく飛ぶことがある。
// そのとき 1 段しか落ちないと、止まっていた分だけゲームが遅れてしまう。
func TestLargeDeltaDropsEveryStepThatFits(t *testing.T) {
	g := New(fixedDraw(O))
	before, _ := g.Active()

	g.Update(3 * FallInterval)

	if after, _ := g.Active(); after.Pos.Y != before.Pos.Y+3 {
		t.Errorf("落下後の行が %d、期待は %d", after.Pos.Y, before.Pos.Y+3)
	}
}

// 大きな dt の途中でロックが起きたら、余った時間はそこで捨てる。捨てないと、
// 前のミノが溜めた時間のせいで新しいミノがいきなり数段落ちる。
func TestLockDiscardsLeftoverFallTime(t *testing.T) {
	g := New(fixedDraw(O, T))
	// O を床に接した状態に置く。次に落とそうとした時点でロックされる。
	g.active = ActiveMino{Kind: O, Rot: Rot0, Pos: Point{X: 3, Y: Height - 2}}

	g.Update(3 * FallInterval)

	next, ok := g.Active()
	if !ok {
		t.Fatal("ロック後に次のミノが出ていない")
	}
	if next.Kind != T {
		t.Fatalf("次のミノが %v、期待は T", next.Kind)
	}
	if next.Pos.Y != spawnPos.Y {
		t.Errorf("新しいミノが前のミノの余り時間で %d 段落ちている", next.Pos.Y-spawnPos.Y)
	}
}

func TestMoveStopsAtTheWall(t *testing.T) {
	g := New(fixedDraw(O))

	for i := 0; i < Width; i++ {
		g.Handle(MoveLeft)
	}
	if m, _ := g.Active(); m.Cells()[0].X != 0 {
		t.Errorf("左端まで寄せたのに一番左のセルが %d 列目にある", m.Cells()[0].X)
	}

	for i := 0; i < 2*Width; i++ {
		g.Handle(MoveRight)
	}
	// O が占めるのは 2 列。右端まで寄せたら 8, 9 列目にいるはず。
	if m, _ := g.Active(); m.Cells()[1].X != Width-1 {
		t.Errorf("右端まで寄せたのに一番右のセルが %d 列目にある", m.Cells()[1].X)
	}
}

// 押しっぱなしは検出できない（keyup が取れない）ので、ソフトドロップは押下 1 回で 1 段。
// 加えて落下タイマーを戻す。戻さないと、自分で落とした直後に重力の分が来て
// 一度の操作で 2 段落ちたように見えることがある。
func TestSoftDropMovesOneStepAndResetsTheTimer(t *testing.T) {
	g := New(fixedDraw(O))
	before, _ := g.Active()

	// 落下間隔の直前まで進めてからソフトドロップする。
	g.Update(FallInterval - time.Millisecond)
	g.Handle(SoftDrop)

	after, _ := g.Active()
	if after.Pos.Y != before.Pos.Y+1 {
		t.Fatalf("ソフトドロップで %d 段落ちた、期待は 1 段", after.Pos.Y-before.Pos.Y)
	}

	// タイマーが戻っていなければ、この 1 ミリ秒で重力の分がもう 1 段来てしまう。
	g.Update(time.Millisecond)
	if now, _ := g.Active(); now.Pos.Y != after.Pos.Y {
		t.Errorf("落下タイマーが戻っていない（さらに %d 段落ちた）", now.Pos.Y-after.Pos.Y)
	}
}

// Input のゼロ値は何もしない。入れ忘れた値が左移動として効いてしまわないこと。
func TestZeroInputDoesNothing(t *testing.T) {
	g := New(fixedDraw(O))
	before, _ := g.Active()

	var unset Input
	g.Handle(unset)

	if after, _ := g.Active(); after != before {
		t.Errorf("何も入っていない入力でミノが %+v から %+v へ動いた", before, after)
	}
}

func TestRotationTurnsTheMino(t *testing.T) {
	g := New(fixedDraw(T))

	g.Handle(RotateCW)

	if m, _ := g.Active(); m.Rot != RotR {
		t.Errorf("回転状態が %v、期待は %v", m.Rot, RotR)
	}
	g.Handle(RotateCCW)
	if m, _ := g.Active(); m.Rot != Rot0 {
		t.Errorf("戻したのに回転状態が %v、期待は %v", m.Rot, Rot0)
	}
}

// M1 のキックテーブルは (0,0) ひとつだけなので、回った先が塞がっていれば回転は起きない
// （→ docs/adr/0002）。M6 で表を差し替えると、ここは「ズレて回る」に変わる。
func TestRotationIsRefusedWhenThereIsNoRoom(t *testing.T) {
	g := New(fixedDraw(T))
	// T を床に接するところへ置く。ここから時計回りに回すと、床の下へ 1 セルはみ出す。
	g.active = ActiveMino{Kind: T, Rot: Rot0, Pos: Point{X: 3, Y: Height - 2}}

	g.Handle(RotateCW)

	if m, _ := g.Active(); m.Rot != Rot0 {
		t.Errorf("回れないはずが %v へ回った", m.Rot)
	}
}

func TestLandingLocksTheMinoAndSpawnsTheNext(t *testing.T) {
	g := New(fixedDraw(O, T))

	dropUntilLocked(t, g)

	want := parseBoard(t, `
		....OO....
		....OO....
	`)
	board := g.Board()
	if got, want := boardArt(&board), boardArt(&want); got != want {
		t.Errorf("ロック後の盤面が違う\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	next, ok := g.Active()
	if !ok {
		t.Fatal("ロック後に次のミノが出ていない")
	}
	if next.Kind != T || next.Pos != spawnPos {
		t.Errorf("次のミノが %v %+v、期待は T %+v", next.Kind, next.Pos, spawnPos)
	}
}

func TestFilledRowIsClearedWhenAMinoLocks(t *testing.T) {
	g := New(fixedDraw(O, T))
	// 最下段を 4〜5 列目だけ空けておく。O がそこへ落ちると 1 行そろう。
	g.board = parseBoard(t, `
		IIII..IIII
	`)

	dropUntilLocked(t, g)

	// そろった行が消え、O の上半分だけが最下段へ落ちてくる。
	want := parseBoard(t, `
		....OO....
	`)
	board := g.Board()
	if got, want := boardArt(&board), boardArt(&want); got != want {
		t.Errorf("ライン消去後の盤面が違う\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGameIsOverWhenTheNextMinoCannotBePlaced(t *testing.T) {
	g := New(fixedDraw(O))

	// 出現位置を塞ぐ。盤面に待避行が無いので、これがそのままゲームオーバーになる。
	for x := 0; x < Width; x++ {
		g.board.cells[0][x] = Cell{Filled: true, Kind: I}
		g.board.cells[1][x] = Cell{Filled: true, Kind: I}
	}
	g.spawn()

	if !g.IsOver() {
		t.Fatal("出現位置が塞がっているのにゲームオーバーになっていない")
	}
	if _, ok := g.Active(); ok {
		t.Error("ゲームオーバー後に操作対象のミノが残っている")
	}
}

func TestNothingHappensAfterGameOver(t *testing.T) {
	g := New(fixedDraw(O))
	g.over = true
	before := g.Board()

	g.Update(10 * FallInterval)
	g.Handle(MoveLeft)
	g.Handle(SoftDrop)

	after := g.Board()
	if boardArt(&before) != boardArt(&after) {
		t.Error("ゲームオーバー後に盤面が動いた")
	}
}

// 盤面の写しを渡しているのは、表示する側に盤面を書き換えさせないため。
func TestBoardReturnsACopy(t *testing.T) {
	g := New(fixedDraw(O))

	copied := g.Board()
	copied.cells[0][0] = Cell{Filled: true, Kind: I}

	if g.board.cells[0][0].Filled {
		t.Error("Board() が返した値を書き換えたら、ゲームの盤面まで変わった")
	}
}
