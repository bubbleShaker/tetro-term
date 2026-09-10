package input

import (
	"testing"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

func TestDecodeSingleKeys(t *testing.T) {
	tests := []struct {
		name  string
		bytes string
		want  []Action
	}{
		{"左矢印", "\x1b[D", []Action{{Input: game.MoveLeft}}},
		{"右矢印", "\x1b[C", []Action{{Input: game.MoveRight}}},
		{"下矢印はソフトドロップ", "\x1b[B", []Action{{Input: game.SoftDrop}}},
		{"上矢印は時計回りの回転", "\x1b[A", []Action{{Input: game.RotateCW}}},
		{"z は反時計回りの回転", "z", []Action{{Input: game.RotateCCW}}},
		{"大文字でも効く", "Z", []Action{{Input: game.RotateCCW}}},
		{"q で終了", "q", []Action{{Quit: true}}},
		{"割り当ての無いキーは無視", "k", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Decoder
			assertActions(t, d.Decode([]byte(tt.bytes)), tt.want)
		})
	}
}

// raw mode では Ctrl-C が SIGINT に変換されず、0x03 という 1 バイトとして届く。
// ここを拾わないと Ctrl-C でゲームから抜け出せなくなる。
func TestCtrlCQuits(t *testing.T) {
	var d Decoder

	assertActions(t, d.Decode([]byte{0x03}), []Action{{Quit: true}})
}

// キーリピートが効いていると、複数のキーが 1 回の読み取りでまとめて届く。
func TestDecodeManyKeysAtOnce(t *testing.T) {
	var d Decoder

	got := d.Decode([]byte("\x1b[D\x1b[D\x1b[Az"))

	assertActions(t, got, []Action{
		{Input: game.MoveLeft},
		{Input: game.MoveLeft},
		{Input: game.RotateCW},
		{Input: game.RotateCCW},
	})
}

// 矢印キーは 3 バイトなので、読み取りの切れ目がその途中に来ることがある。
// 1 バイトずつ届けても、つながって 1 つの操作になること。
func TestDecodeSequenceArrivingInPieces(t *testing.T) {
	var d Decoder

	for i, b := range []byte("\x1b[D") {
		got := d.Decode([]byte{b})
		if i < 2 {
			// まだ全部届いていないので、何も決まらない。
			assertActions(t, got, nil)
			continue
		}
		assertActions(t, got, []Action{{Input: game.MoveLeft}})
	}
}

func TestDecodeSequenceSplitInTheMiddle(t *testing.T) {
	var d Decoder

	assertActions(t, d.Decode([]byte("\x1b[C\x1b")), []Action{{Input: game.MoveRight}})
	assertActions(t, d.Decode([]byte("[C")), []Action{{Input: game.MoveRight}})
}

// ESC を単体で押しても、次に押したキーが飲み込まれないこと。
// ESC を溜め込んだままにすると、その後のキーが全部読めなくなる。
func TestLoneEscapeDoesNotSwallowTheNextKey(t *testing.T) {
	var d Decoder

	assertActions(t, d.Decode([]byte{escByte}), nil)
	assertActions(t, d.Decode([]byte("q")), []Action{{Quit: true}})
}

// 割り当ての無いエスケープ（ファンクションキーなど）を読み飛ばしたあとも、
// 続きのキーが読めること。
func TestUnknownEscapeSequenceIsSkipped(t *testing.T) {
	var d Decoder

	got := d.Decode([]byte("\x1b[Hq"))

	assertActions(t, got, []Action{{Quit: true}})
}

// 持ち越しが無いのに内側の配列だけが育ち続けないこと。
// 1 秒に何十回も呼ばれるので、ここが積み上がると遊んでいる間ずっと太り続ける。
func TestPendingDoesNotGrowWhenEverythingIsConsumed(t *testing.T) {
	var d Decoder

	for i := 0; i < 100; i++ {
		d.Decode([]byte("\x1b[D"))
	}

	if len(d.pending) != 0 {
		t.Errorf("読み切ったのに %d バイト残っている", len(d.pending))
	}
}

func assertActions(t *testing.T, got, want []Action) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("操作が %d 個（%+v）、期待は %d 個（%+v）", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%d 個目が %+v、期待は %+v", i, got[i], want[i])
		}
	}
}
