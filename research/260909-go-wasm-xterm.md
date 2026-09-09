# Go を WebAssembly にして xterm.js で動かす — 調査記録

2026-09-09。M0 の実装にあたって実測した事実をまとめる。推測ではなく、
すべてこの環境で動かして確かめた結果。

## ツールチェーン

- Go は公式 tarball を `~/.local/go-sdk/go` に展開（**go1.27.1**）。apt の
  `golang-go` は 1.22 と古かったため使わなかった。
- `GOOS=js GOARCH=wasm` は go1.27.1 でも健在。標準の `syscall/js` が使える。
- 起動用グルーコード `wasm_exec.js` の置き場は **`$(go env GOROOT)/lib/wasm/`**。
  かつての `misc/wasm/` ではない。Go のバージョンと厳密に対応するため、
  リポジトリにコミットせずビルド時にコピーする方針にした。

### バイナリサイズ（実測）

`cmd/tetro-wasm`（アスキーアートを1回書くだけ）で **2.5MB**。
Go ランタイムがまるごと入るため、内容をどれだけ削ってもこの程度が下限になる。
gzip 転送で 600KB〜1MB 程度。TinyGo なら桁違いに小さくできるが、
goroutine や reflect に制約が出るので、実際に「重くて遊べない」と分かるまでは使わない。

## `main` を終了させない

WASM は `main` が return するとインスタンスごと終了し、以降 JS から呼び戻せない。
Go の WebAssembly wiki で紹介されている `<-make(chan struct{})` で待機させる。

**デッドロック検出に引っかからないか**が気になったので実際に確かめた。
node 上で 1.5 秒待っても `go.run()` の Promise は解決せず、
`all goroutines are asleep - deadlock!` も出なかった。**問題なく待機し続ける**。

## xterm.js

- `@xterm/xterm` 6.0.0 と `@xterm/addon-fit` 0.11.0 を npm から取得し、
  **ESM 版（`xterm.mjs` 337KB）をリポジトリに同梱**した。ESM なのでバンドラ不要で
  `import` できる。CDN を参照しないので GitHub Pages でも外部依存ゼロ。
- `addon-fit.mjs` は外部 import を一切持たない（確認済み）ので、そのまま置ける。
- 既定は DOM レンダラで、`canvas` 要素は作られない（`.xterm-rows > div` に行が並ぶ）。

### `convertEol` は使わない

`convertEol: true` にすると xterm 側が `\n` を CRLF に読み替えてくれるが、
これを使うと「改行の形をどちらが決めるか」が曖昧になる。ADR-0001 のとおり
フレーム文字列がコアと表示の唯一の境界なので、**Go 側が CRLF を吐く**ことにして
`convertEol: false` のままにした。

### `instantiateStreaming` を避けた

`WebAssembly.instantiateStreaming` は Content-Type が `application/wasm` でないと失敗する。
手元の `python3 -m http.server` で配信すると条件を満たさないことがあるため、
`fetch` → `arrayBuffer` → `WebAssembly.instantiate` の経路にした。

## この WSL 環境での落とし穴

### headless Chromium では requestAnimationFrame が発火しない

Playwright の headless Chromium で `requestAnimationFrame` が**まったく呼ばれない**。
そのため:

- xterm.js は描画を rAF で回すので、**文字を書いても画面に出ない**
  （`term.write()` を JS から直接叩いても同じ。Go 側の問題ではない）
- Playwright の `page.waitForFunction` は既定で rAF ポーリングなので**必ずタイムアウトする**

`--disable-background-timer-throttling` などのフラグを足しても解消しなかった。

### 描画の代わりに何を検証したか

`page.addInitScript` で `globalThis.tetroTerm` の代入を捕まえ、`write` を包んで
渡された文字列を記録した。これで **Go → WASM → JS → xterm.write の経路**を
実ブラウザ内で観測でき、CRLF・ANSI の色指定・Go のバージョンが正しく届くことを確認した。
**グリフが実際に描かれるかだけは実機で確認する**しかない。

### ヘッドフルも動かなかった

`DISPLAY=:0` と `/tmp/.X11-unix/X0` は存在する（WSLg）が、
Chromium は `Missing X server or $DISPLAY` で起動しなかった。

### スクリーンショットはフォント待ちで固まる

`page.screenshot()` が "waiting for fonts to load..." の直後で固まる。
CDP の `Page.captureScreenshot` を直接叩けば取得できる。

### /mnt/c 上では git が実行ビットを記録しない

`chmod +x` しても git には `100644` で入り、CI で `./scripts/build-wasm.sh` が
Permission denied になる。`git update-index --chmod=+x <path>` で明示的に立てる。

## CI での罠

ホスト（linux/amd64）で `go vet ./...` / `go test ./...` を走らせると、
`//go:build js && wasm` のせいで**対象パッケージが 0 個になり終了コード 1 で落ちる**。

```
go: warning: "./..." matched no packages
no packages to test    → exit 1
```

`GOOS=js GOARCH=wasm go vet ./...` なら対象が見つかり exit 0 になる。
コアを追加する M1 以降はホスト側にもパッケージができるので `go test ./...` が使える。
