package input

import (
	"testing"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

func TestPressResolvesEveryButton(t *testing.T) {
	for _, b := range Buttons() {
		in, ok := Press(b.ID)
		if !ok {
			t.Errorf("ボタン %q が押されても読み替えられない", b.ID)
			continue
		}
		if in == game.InputNone {
			t.Errorf("ボタン %q が「何もしない」に割り当たっている", b.ID)
		}
	}
}

func TestPressRejectsAnUnknownID(t *testing.T) {
	if _, ok := Press("hold"); ok {
		t.Error("割り当ての無い id が入力として通ってしまった")
	}
}

func TestButtonsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range Buttons() {
		if seen[b.ID] {
			t.Errorf("id %q が 2 つある。片方は決して押せない", b.ID)
		}
		seen[b.ID] = true

		if b.Label == "" || b.What == "" {
			t.Errorf("ボタン %q に文字が無い（label=%q what=%q）", b.ID, b.Label, b.What)
		}
		if b.Side != SideLeft && b.Side != SideRight {
			t.Errorf("ボタン %q の置き場所が %q で、左右のどちらでもない", b.ID, b.Side)
		}
	}
}

// 返した一覧を書き換えても、次に取ったものが汚れていないこと。
func TestButtonsAreNotSharedWithTheCaller(t *testing.T) {
	got := Buttons()
	if len(got) == 0 {
		t.Fatal("ボタンが 1 つも無い")
	}
	got[0].Label = "書き換え"

	if Buttons()[0].Label == "書き換え" {
		t.Error("受け取った側の書き換えが表そのものに届いている")
	}
}

// 押しっぱなしで繰り返してよいのは移動だけであること。
//
// 回転が繰り返すと、指を置いている間ずっと回り続けて狙った向きで止められない。
// ロックディレイ（M4 / #5）が入ると、接地したまま回し続けて無限に粘れることにもなる。
func TestOnlyMovementRepeats(t *testing.T) {
	mayRepeat := map[game.Input]bool{
		game.MoveLeft:  true,
		game.MoveRight: true,
		game.SoftDrop:  true,
	}

	for _, b := range buttons {
		if b.Repeat && !mayRepeat[b.input] {
			t.Errorf("ボタン %q は押しっぱなしで繰り返す設定だが、繰り返してよい操作ではない", b.ID)
		}
	}
}

// キーで押せる操作は、ボタンでも押せること。
//
// **これが M4・M5 の取りこぼしを止める**。ハードドロップやホールドをキーに足したとき、
// ボタンを足し忘れてもプログラムは動き続け、PC では誰も気づかない。気づくのは
// スマホで遊んで「落とせない」と分かったときになる。ここで落とす。
//
// 終了（q / Ctrl-C）を除くのは、それがゲームへの入力ではなくドライバへの合図であり、
// ブラウザ版では何も起きないためである（→ Action）。
func TestEveryKeyInputIsAlsoAButton(t *testing.T) {
	onAButton := map[game.Input]bool{}
	for _, b := range buttons {
		onAButton[b.input] = true
	}

	for _, table := range []map[byte]Action{singleByte, arrow} {
		for key, action := range table {
			if action.Input == game.InputNone {
				continue
			}
			if !onAButton[action.Input] {
				t.Errorf("キー %q の操作 %d にボタンが無い。スマホではこの操作ができない",
					string(rune(key)), action.Input)
			}
		}
	}
}

// 逆向き。どのキーにも割り当たっていない操作をボタンだけが持っていないこと。
// 片側だけで遊べる操作があると、CLI とブラウザで遊び方が食い違う。
func TestEveryButtonInputIsAlsoAKey(t *testing.T) {
	onAKey := map[game.Input]bool{}
	for _, table := range []map[byte]Action{singleByte, arrow} {
		for _, action := range table {
			onAKey[action.Input] = true
		}
	}

	for _, b := range buttons {
		if !onAKey[b.input] {
			t.Errorf("ボタン %q の操作 %d は、どのキーにも割り当たっていない", b.ID, b.input)
		}
	}
}
