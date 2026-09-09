# tetro-term

ターミナルで動く落ち物パズル。Go で書かれていて、**同じコードが CLI とブラウザの両方で動く**。

ブラウザ版は Go を WebAssembly にコンパイルし、その端末出力を
[xterm.js](https://xtermjs.org/) がそのまま描画している。
つまりブラウザに見えているのは端末エミュレータの画面そのもの。

🎮 **[ブラウザで遊ぶ](https://bubbleshaker.github.io/tetro-term/)** （M0 進行中）

## ターミナルで遊ぶ

```sh
go run ./cmd/tetro
```

## 操作

| キー | 動作 |
| --- | --- |
| `←` `→` | 左右に移動 |
| `↓` | ソフトドロップ |
| `Space` | ハードドロップ |
| `↑` / `x` | 右回転 |
| `z` | 左回転 |
| `c` | ホールド |
| `q` | 終了 |

スマートフォンでは画面下のボタンで操作する。

### 押しっぱなしの移動について

端末も xterm.js も「キーが離された」ことをアプリに伝えてくれないため、
押しっぱなしの連続移動は **OS のキーリピート設定に委ねている**。
移動が遅い・速すぎると感じる場合は OS 側のキーリピート設定を調整してほしい。

## 開発

設計と段取りは [`PLAN.md`](./PLAN.md)、用語は [`CONTEXT.md`](./CONTEXT.md)、
覆すのが高くつく決定は [`docs/adr/`](./docs/adr/) にある。

```sh
go test ./...                        # コアのテスト
go run ./cmd/tetro                   # CLI 版
GOOS=js GOARCH=wasm go build -o web/main.wasm ./cmd/tetro-wasm   # ブラウザ版
```

## ライセンス

MIT。

本作は落ち物パズルの独自実装であり、特定の商用作品とは無関係。
