//go:build !js

// tetro は本物のターミナルで遊ぶための入口。
//
// ここにゲームのルールは無い（→ CONTEXT.md「ドライバ」）。この main がやるのは
// 端末を整えること、時間を測ってコアを進めること、キーを入力に読み替えること、
// フレームを画面へ流すこと、そして**必ず端末を元に戻して終わること**だけである。
//
// ブラウザには端末もシグナルも無い（syscall.SIGHUP がそもそも存在しない）ので、
// この入口は js/wasm 向けにはビルドしない。ブラウザ側の入口は cmd/tetro-wasm にある。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/bubbleShaker/tetro-term/internal/draw"
	"github.com/bubbleShaker/tetro-term/internal/game"
	"github.com/bubbleShaker/tetro-term/internal/input"
	"github.com/bubbleShaker/tetro-term/internal/render"
)

// tickInterval はコアを進める間隔。
//
// 落下の速さではない（それはコアが落下間隔として持っている）。ここで決めているのは
// 「どれくらい細かく時間を刻んでコアに渡すか」で、細かいほど操作が滑らかに見える。
// 30ms なら 1 秒に約 33 回で、人が押した瞬間との差は気づけない程度に収まる。
const tickInterval = 30 * time.Millisecond

func main() {
	// 後始末は run の中の defer が引き受ける。os.Exit は defer を飛ばすので、
	// **終了コードを決めるのは run から戻ったあと**でなければならない。
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "tetro:", err)
		os.Exit(1)
	}
}

func run() error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return errors.New("標準入力が端末ではない。ターミナルから直接起動すること")
	}

	// raw mode にすると、キーが押されるたびに 1 文字ずつ届くようになる（普段は Enter を
	// 押すまでまとめて待たされる）。同時にエコーも止まるので、押したキーが盤面の上に
	// 表示されることもなくなる。
	state, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("端末を raw mode にできない: %w", err)
	}
	// **あらゆる終了経路がここを通る**。通らずに終わると、端末はキーをエコーしないまま
	// 返り、ユーザーのシェルが壊れたように見える。panic で巻き戻るときも通る。
	defer func() {
		if err := term.Restore(fd, state); err != nil {
			// もう画面は当てにできないので標準エラーへ。黙って戻せないまま終わると、
			// ユーザーは何が起きたか分からないまま壊れた端末を渡される。
			fmt.Fprintln(os.Stderr, "tetro: 端末を元に戻せなかった:", err)
		}
	}()

	// Leave は raw mode のうちに書く必要がある（改行が CRLF でないと行頭に戻らない）。
	// defer は後入れ先出しなので、Restore より後に登録したこちらが先に走る。
	write(render.Enter())
	defer write(render.Leave())

	// raw mode ではキーからシグナルが起きなくなる（Ctrl-C は 1 バイトとして届くので
	// Decoder が拾う）。ここで待つのは、外から kill されたときのぶんである。
	// 拾わずに殺されると、やはり端末が戻らないまま残る。
	//
	// SIGKILL は捕まえられないので、それで殺された場合だけは端末が戻らない。
	// SIGTSTP（一時停止）も扱っていない。止まっている間 raw mode のままになるが、
	// 正しく直すには停止と再開の両方で端末を付け替える必要があり、M1 では見送る。
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, os.Interrupt)
	defer signal.Stop(signals)

	return play(signals)
}

// play はゲームが終わるまで回り続ける。
func play(signals <-chan os.Signal) error {
	g := game.New(draw.Uniform())

	keys := readAll(os.Stdin)
	var decoder input.Decoder

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	// コアへ渡すのは「前に進めたときから実際に何秒経ったか」であって、刻みの設定値ではない。
	// time.Ticker は受け取りが遅れると刻みを捨てるので、設定値を渡し続けると
	// ゲームの中の時間が実時間より遅れていく。
	last := time.Now()

	for {
		write(render.Frame(g))

		// 「キーが押される」と「時間が経つ」は、どちらもいつ来るか分からない。
		// select はその 2 つを 1 か所で待つための仕組みで、先に来たほうを処理する。
		select {
		case <-signals:
			return nil

		case chunk, ok := <-keys:
			if !ok {
				return nil // 標準入力が閉じた
			}
			for _, action := range decoder.Decode(chunk) {
				if action.Quit {
					return nil
				}
				g.Handle(action.Input)
			}

		case now := <-ticker.C:
			g.Update(now.Sub(last))
			last = now
		}
	}
}

// readAll は標準入力を読み続け、届いたバイト列を流す channel を返す。
//
// 読み取りは何か届くまで戻ってこない。そのまま呼ぶと、キーが押されない限り
// 時間を測れず、ミノが落ちなくなる。別の goroutine に切り離して、
// 待つ場所を select ひとつに集めている。
//
// この goroutine は終わらせない。ゲームを抜けるとき、読み取りの途中で止まったまま
// 残ることになるが、直後にプロセスごと終わるので回収の必要がない。**止めようとする方が
// 危ない**——読み取りを中断する手立ては端末を閉じることで、それは端末を元に戻す前の
// 状態を壊しかねない。
func readAll(r io.Reader) <-chan []byte {
	chunks := make(chan []byte)

	go func() {
		defer close(chunks)

		buf := make([]byte, 64)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				// buf は次の読み取りで上書きされるので、渡す前に複製する。
				// そのまま送ると、受け取った側が読む前に中身が変わりうる。
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				chunks <- chunk
			}
			if err != nil {
				return
			}
		}
	}()

	return chunks
}

// write はフレームを画面へ流す。
//
// 書き込みの失敗は無視する。画面へ書けない状況ではもう伝える手立てが無く、
// ここで止めるとかえって端末を戻さずに終わることになる。
func write(s string) {
	_, _ = io.WriteString(os.Stdout, s)
}
