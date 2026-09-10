package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/bubbleShaker/tetro-term/internal/game"
)

// painted はフレームの中の 1 文字と、そこで効いている装飾。
type painted struct {
	ch rune
	st style
}

// paintedRunes はフレームを読み戻して、1 文字ずつ「そこで何色が効いているか」を返す。
//
// 出来上がった文字列をただ目視するのではなく**解釈し直している**のは、
// 「塗られたセル」と「空のセル」がどちらも半角スペースで、文字だけを見ても
// 区別がつかないからである。色を追いかけて初めて盤面の形が見える。
//
// 装飾を捨てて絵だけを作るのでは足りない。色が枠まで漏れていても絵の形は変わらず、
// 「テストは通るのに画面は壊れている」が起きる。だから装飾を持ったまま返し、
// 検査する側が形と色のどちらでも問い詰められるようにしてある。
func paintedRunes(frame string) []painted {
	var out []painted
	st := plain

	for _, tok := range tokenize(frame) {
		if tok.escape {
			st = applySGR(st, tok.text)
			continue
		}
		for _, r := range tok.text {
			if r == '\r' {
				// CRLF の CR は絵には要らない。
				continue
			}
			out = append(out, painted{ch: r, st: st})
		}
	}
	return out
}

// frameArt は paintedRunes を目で読める絵に畳む。
//
// 塗られているマスは '#'、塗られていないマスは '.'、枠と重ね書きの文字はそのまま。
// 文字色が指定されている空白だけは '#' にせずそのまま空白にする。文字色を使うのは
// 重ね書きの文字だけなので、これで "GAME OVER" の中の空白が塗りつぶしと混ざらない。
func frameArt(frame string) string {
	var sb strings.Builder
	for _, p := range paintedRunes(frame) {
		switch {
		case p.ch != ' ':
			sb.WriteRune(p.ch)
		case p.st.fg != noColor:
			sb.WriteRune(' ')
		case p.st.bg != noColor:
			sb.WriteRune('#')
		default:
			sb.WriteRune('.')
		}
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

type token struct {
	text   string
	escape bool
}

var escapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func tokenize(s string) []token {
	var tokens []token
	last := 0
	for _, loc := range escapePattern.FindAllStringIndex(s, -1) {
		if loc[0] > last {
			tokens = append(tokens, token{text: s[last:loc[0]]})
		}
		tokens = append(tokens, token{text: s[loc[0]:loc[1]], escape: true})
		last = loc[1]
	}
	if last < len(s) {
		tokens = append(tokens, token{text: s[last:]})
	}
	return tokens
}

// applySGR は色を指定するエスケープだけを解釈し、装飾の変化を返す。
// カーソル移動や画面消去は絵の形に関わらないので無視する。
func applySGR(st style, escape string) style {
	if !strings.HasSuffix(escape, "m") {
		return st
	}
	params := strings.Split(strings.TrimSuffix(strings.TrimPrefix(escape, esc), "m"), ";")
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case "0", "":
			st = plain
		case "48", "38":
			// 48;5;N が背景色、38;5;N が文字色。
			if i+2 >= len(params) || params[i+1] != "5" {
				continue
			}
			n, err := strconv.Atoi(params[i+2])
			if err != nil {
				continue
			}
			if params[i] == "48" {
				st.bg = n
			} else {
				st.fg = n
			}
			i += 2
		}
	}
	return st
}

// boardLines はフレームの絵から、上下の枠と左右の枠を取り除いた盤面 20 行だけを返す。
func boardLines(t *testing.T, frame string) []string {
	t.Helper()

	lines := strings.Split(frameArt(frame), "\n")
	if len(lines) != game.Height+2 {
		t.Fatalf("フレームが %d 行ある（枠 2 行 + 盤面 %d 行のはず）", len(lines), game.Height)
	}

	inner := make([]string, 0, game.Height)
	for i, line := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			t.Fatalf("盤面 %d 行目が縦の枠に挟まれていない: %q", i, line)
		}
		inner = append(inner, line[1:len(line)-1])
	}
	return inner
}

// newGame は決まった順にミノが出るゲームを作る。
func newGame(kinds ...game.MinoKind) *game.Game {
	i := 0
	return game.New(func() game.MinoKind {
		k := kinds[i]
		if i < len(kinds)-1 {
			i++
		}
		return k
	})
}

// lockOne はアクティブミノがロックされるまでソフトドロップする。
func lockOne(t *testing.T, g *game.Game) {
	t.Helper()

	before, ok := g.Active()
	if !ok {
		t.Fatal("すでにゲームオーバーになっている")
	}
	for i := 0; i < game.Height+4; i++ {
		g.Handle(game.SoftDrop)
		if now, ok := g.Active(); !ok || now.Pos.Y <= before.Pos.Y {
			return
		}
	}
	t.Fatal("ミノがロックされなかった")
}

// newGameOver は O ミノを積み上げて、実際にゲームオーバーまで進めたゲームを返す。
//
// O は 4〜5 列目に出て 2 行ぶん積むので、列がそろわないまま天井まで届く。
// 盤面を外から作らずに本物の手順で詰ませているので、表示だけでなく
// ゲームオーバーの成立条件そのものも一緒に確かめられる。
func newGameOver(t *testing.T) *game.Game {
	t.Helper()

	g := newGame(game.O)
	for i := 0; i < game.Height; i++ {
		if g.IsOver() {
			return g
		}
		lockOne(t, g)
	}
	t.Fatal("O を積み上げてもゲームオーバーにならなかった")
	return nil
}
