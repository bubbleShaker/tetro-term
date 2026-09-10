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

// 端末が application cursor key mode に入っていると、矢印キーは ESC [ ではなく
// ESC O で始まる。片方しか見ないと設定次第で矢印が全部効かなくなる。
func TestArrowKeysInApplicationCursorMode(t *testing.T) {
	tests := map[string]Action{
		"\x1bOD": {Input: game.MoveLeft},
		"\x1bOC": {Input: game.MoveRight},
		"\x1bOB": {Input: game.SoftDrop},
		"\x1bOA": {Input: game.RotateCW},
	}

	for bytes, want := range tests {
		var d Decoder
		assertActions(t, d.Decode([]byte(bytes)), []Action{want})
	}
}

// 修飾キー付きの矢印は ESC [ 1 ; 5 D のように 3 バイトを超える。
// 長さを決め打ちにすると余りが出て、その残骸が別のキーとして読まれる。
func TestModifiedArrowIsConsumedWhole(t *testing.T) {
	var d Decoder

	// Ctrl+← のあとに q。q だけが読めること（残骸から幽霊の操作が湧かないこと）。
	got := d.Decode([]byte("\x1b[1;5Dq"))

	assertActions(t, got, []Action{{Quit: true}})
}

// 終わりの来ない並びを渡されても、持ち越しが際限なく膨らまないこと。
func TestPendingIsBoundedForANeverEndingSequence(t *testing.T) {
	var d Decoder

	for i := 0; i < 100; i++ {
		// 引数バイトばかりで終端バイトが来ない並び。
		d.Decode([]byte("\x1b[123456789"))
	}

	if len(d.pending) > maxSequence {
		t.Errorf("持ち越しが %d バイトまで膨らんだ（上限は %d）", len(d.pending), maxSequence)
	}
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
