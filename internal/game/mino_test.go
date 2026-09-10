package game

import "testing"

// 28 個の形は手で書き下ろした表なので、書き間違いをここで捕まえる。
// 形そのものの正しさ（T が T の字に見えるか）は目で見るしかないが、
// 「4 セルある」「箱からはみ出していない」「重複していない」は機械が確かめられる。
func TestShapesAreWellFormed(t *testing.T) {
	for _, kind := range Kinds {
		for rot := range shapes[kind] {
			shape := shapes[kind][rot]
			seen := map[Point]bool{}
			for _, p := range shape {
				if p.X < 0 || p.X >= boxSize || p.Y < 0 || p.Y >= boxSize {
					t.Errorf("%v %v: セル %+v が %dx%d の箱の外にある",
						kind, Rotation(rot), p, boxSize, boxSize)
				}
				if seen[p] {
					t.Errorf("%v %v: セル %+v が重複している", kind, Rotation(rot), p)
				}
				seen[p] = true
			}
		}
	}
}

// O は回転しても動いてはならない。この性質があるからこそ、回転を実行時の
// 座標変換ではなく書き下ろした表にしている（→ mino.go の shapeArt のコメント）。
func TestOMinoDoesNotMoveWhenRotated(t *testing.T) {
	base := shapes[O][Rot0]
	for _, rot := range []Rotation{RotR, Rot2, RotL} {
		if shapes[O][rot] != base {
			t.Errorf("O の %v が %v と違う形になっている: %+v", rot, Rot0, shapes[O][rot])
		}
	}
}

func TestRotationCyclesBackToStart(t *testing.T) {
	got := Rot0
	for i := 0; i < 4; i++ {
		got = got.CW()
	}
	if got != Rot0 {
		t.Errorf("時計回りに 4 回で %v に戻るべきだが %v になった", Rot0, got)
	}

	if got := Rot0.CCW(); got != RotL {
		t.Errorf("%v から反時計回りは %v のはずだが %v になった", Rot0, RotL, got)
	}
	if got := Rot0.CW().CCW(); got != Rot0 {
		t.Errorf("時計回りと反時計回りが打ち消し合っていない: %v", got)
	}
}

// 出現位置は「3 セル幅のミノが 3〜5 列目、I ミノが 3〜6 列目」に出ること。
func TestSpawnPositionIsCentered(t *testing.T) {
	tests := []struct {
		kind      MinoKind
		wantCells [4]Point
	}{
		{I, [4]Point{{3, 1}, {4, 1}, {5, 1}, {6, 1}}},
		{O, [4]Point{{4, 0}, {5, 0}, {4, 1}, {5, 1}}},
		{T, [4]Point{{4, 0}, {3, 1}, {4, 1}, {5, 1}}},
	}
	for _, tt := range tests {
		m := ActiveMino{Kind: tt.kind, Rot: Rot0, Pos: spawnPos}
		if got := m.Cells(); got != tt.wantCells {
			t.Errorf("%v の出現位置が %+v、期待は %+v", tt.kind, got, tt.wantCells)
		}
	}
}
