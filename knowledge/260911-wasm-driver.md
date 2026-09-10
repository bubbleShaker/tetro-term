# M2 で出てきた概念 — Go と JS をどう繋いだか

「なぜこう書いてあるのか」が分かりにくいところだけを拾う。
何をしたかの全体像は [`summary/260911-m2.md`](../summary/260911-m2.md) にある。

## `js.FuncOf` — Go の関数を JS から呼べるようにする

Go の関数はそのままでは JS から呼べない。`js.FuncOf` で包むと、JS 側から見える
関数オブジェクト（`js.Func`）になる。

```go
tick := js.FuncOf(func(this js.Value, args []js.Value) any {
    d.tick(args[0].Float())  // JS から渡された最初の引数を float64 として読む
    return nil               // JS へ返す値。nil は undefined
})
```

引数が `[]js.Value` なのは、JS には「引数の個数と型が合っているか」を保証する
仕組みが無いからである。**`args[0]` は無いかもしれない**。無いのに触ると Go が
panic し、panic は WASM インスタンスごと巻き込むので、以降ゲームが二度と動かない。
`cmd/tetro-wasm/main.go` の `callback` が引数の個数を確かめているのはそのため。

### Release を呼んでいない理由

`js.Func` は Go 側に参照を残すので、本来は使い終わったら `Release()` が要る。
呼ばないと JS 側が参照を捨てても Go 側のオブジェクトが残り続ける（リーク）。

このプロジェクトでは呼んでいない。**解放してよいのは「もう呼ばれない」と決まった
瞬間**だが、`tick` と `data` はタブが閉じるまで呼ばれ続けるので、その瞬間が来ない。
タブが閉じれば WASM インスタンスごと消える。

## `main` が return してはいけない

```go
<-make(chan struct{})  // 誰も送ってこない channel から受け取ろうとして、永久に止まる
```

普通の Go プログラムは `main` が返れば終わる。WASM も同じで、**返るとインスタンスが
終了し、JS から `tick` を呼んでも「もう死んでいる」というエラーになる**。
ゲームループは JS 側にあるので、Go 側は起動処理を終えたあと**何もせずに生き続ける**
必要がある。上の 1 行はそのための定型句。

`web/main.js` が `go.run(instance).then(() => fail(...))` と書いてあるのは、
この前提の裏返しである。**解決したらそれは異常終了を意味する**。

## `requestAnimationFrame` の経過時間

```js
let last = performance.now();
function step(now) {
  tick(now - last);   // 前回からの経過ミリ秒
  last = now;
  requestAnimationFrame(step);
}
```

`requestAnimationFrame(step)` は「次に画面を描くとき `step` を呼んでくれ」という
予約である。1 回きりなので、**続けるには中でもう一度予約する**。
引数の `now` は `performance.now()` と同じ時刻（ページを開いてからのミリ秒）。

### なぜ `setInterval` ではないのか

背面タブでの扱いが違う。

| | 背面タブ |
| --- | --- |
| `setInterval` / `setTimeout` | **1 秒に 1 回まで絞られる**（止まらない） |
| `requestAnimationFrame` | **止まる** |

止まらないほうが良さそうに見えるが、このゲームでは逆である。コアは
「渡された経過時間の分だけ落とす」ので、絞られている間に溜まった時間を渡すと、
タブに戻った瞬間にミノが何段も落ちている。rAF なら「見ていない間は進まない」。

### それでも上限が要る

rAF が止まるということは、**戻ってきた最初の 1 回の経過時間が数分になりうる**という
ことでもある。`clock.go` が 100ms で切り詰めている。

## CSS `transform: scale()` で端末を拡縮する

xterm.js の端末は「22 桁 × 22 行」で作ってある。この大きさはフォントサイズで決まる
実寸を持つので、画面が狭いとはみ出す。そこで要素ごと拡縮する。

```js
const scale = Math.min(
  stageEl.clientWidth / terminalEl.offsetWidth,
  stageEl.clientHeight / terminalEl.offsetHeight,
);
terminalEl.style.transform = `scale(${scale})`;
```

**`offsetWidth` / `offsetHeight` は `transform` の影響を受けない**（レイアウト上の
実寸が取れる）。これを知らないと「拡大したら実寸も大きくなって、次の計算がずれる」
という無限ループを書くことになる。`getBoundingClientRect()` のほうは拡縮後の値を返す。

`transform` は**レイアウトを変えない**——要素は元の大きさの場所を占めたまま、
見た目だけ伸び縮みする。だから ResizeObserver で観測しても、拡縮が自分自身を
呼び戻すことはない。文字は再ラスタライズされるのでぼやけない。

### ResizeObserver

「この要素の大きさが変わったら教えてくれ」を登録する仕組み。`window` の `resize`
イベントと違い、**窓の大きさが変わっていなくても**（隣の文字が折り返して行数が増えた、
フォントが確定した、など）大きさが変われば呼ばれる。

このページでは入れ物（`#stage`）と端末の**両方**を観測している。片方しか見ていないと、
もう片方が動いたときに倍率が古いまま残る。

## `//go:build js && wasm` とビルドタグ

ファイルの先頭にこれを書くと、**そのファイルは js/wasm 向けにビルドするときだけ
コンパイルされる**。`cmd/tetro-wasm/main.go` は `syscall/js` を使っており、
これは js/wasm でしか存在しないパッケージなので、タグが無いとホストで
`go build` した瞬間に落ちる。

### タグの無いファイルを混ぜると、そこだけテストできる

同じディレクトリにタグの無いファイルを置くと、そのファイルは**どの環境でも**
コンパイルされる。`clock.go` と `screen.go` がそれで、ホストの `go test` から見える。

```
cmd/tetro-wasm/
├── main.go        //go:build js && wasm   ← ホストからは存在しないのと同じ
├── clock.go       （タグ無し）            ← テストできる
├── screen.go      （タグ無し）            ← テストできる
└── *_test.go      （タグ無し）
```

`func main` が無い `package main` になるが、**`go test` と `go vet` は通る**
（テストバイナリは自前の `main` を持つため）。落ちるのは `go build ./...` だけで、
CI はこれを実行していない。

「js/wasm だからテストできない」の範囲は思っているより狭い、というのがここの教訓。
ブラウザに触らない判断——時間をどう進めるか、いつ画面を送るか——は分けて置ける。

## Go から JS のオブジェクトを作る

```go
t.obj.Call("ready", map[string]any{
    "cols": s.cols,
    "tick": s.tick,   // js.Func もそのまま入る
})
```

`js.ValueOf` が変換できる型は決まっている（`bool` / 数値 / `string` / `nil` /
`js.Value` / `js.Func` / `[]any` / `map[string]any`）。**構造体は変換できない**ので、
`map[string]any` に詰め替える。`map[string]any` は JS のオブジェクトになる。

`cmd/tetro-wasm/main.go` で `setup` 構造体を作ってから詰め替えているのは、
呼び出し側が引数の順番を取り違えないようにするため。JS 側のキー名との対応が
1 か所（`ready` メソッドの中）に閉じる。

## この WSL でブラウザ確認をするときの注意

**headless Chromium は描画ライフサイクルを回さない。** rAF が 1 秒間に 0 回しか
発火せず、スクリーンショットも取れない。つまり**画面を見る形の確認ができない**。

M2 では、`globalThis.tetroTerm` への代入をテスト側から横取りして
`write` / `ready` / `gameOver` を包み、**Go が書き出したフレーム文字列を直接検査する**
方式で 31 項目を自動化した。`tick(経過ミリ秒)` をテストから直接呼べるので、
時間を待たずに重力もクランプも確かめられる。production のコードには手を入れていない。

この方式でも確かめられないのは、rAF ループが実際に回ることと ResizeObserver の
追従だけ。そこは実機で見るしかない。
