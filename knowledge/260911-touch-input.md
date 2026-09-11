# M3 で出てきた概念 — ブラウザで指を扱う

「なぜこう書いてあるのか」が分かりにくいところだけを拾う。
何をしたかの全体像は [`summary/260911-m3.md`](../summary/260911-m3.md) にある。

## ポインタイベント — マウスと指を同じ口で受ける

昔は `mousedown` と `touchstart` を別々に書く必要があった。いまは **PointerEvent**
という後から入った仕組みがあり、マウス・指・ペンのどれでも `pointerdown` で届く。
どれだったかは `event.pointerType`（`"mouse"` / `"touch"` / `"pen"`）で分かる。

`web/main.js` が「指で触られたか」を見ているのはこれである。

```js
document.addEventListener("pointerdown", (e) => {
  if (e.pointerType === "touch") enableTouchMode();
}, { capture: true });
```

`{ capture: true }` は**捕捉相**で受け取るという指定。イベントは
「窓 → 目的の要素」（捕捉相）→「目的の要素 → 窓」（浮上相）の順に流れる。
既定は浮上相なので、途中の要素が `stopPropagation()` で止めると届かない。
ボタン側が `preventDefault()` するので、こちらは捕捉相にして必ず先に通す。

## `setPointerCapture` — 指を捕まえておく

押したまま指がボタンの外へ滑ると、既定では `pointerup` が**別の要素**に届く。
押しっぱなしの繰り返しを止めるのは `pointerup` なので、これを取り逃がすと
指を離してもミノが走り続ける。

```js
el.setPointerCapture(e.pointerId);
```

こう呼んでおくと、その指の続きのイベントは全部この要素へ届く。
`pointerId` は指ごとに振られる番号で、2 本指で別々のボタンを押したときに
どちらの続きかを見分けるためにある。

（タッチの場合は仕様上そもそも暗黙に捕まっていることが多いが、マウスやペンでは
そうならない。明示的に呼んでおけば、どの入力装置でも同じ動きになる。）

## `touch-action` — スクロールや拡大に先回りする

ブラウザは指の動きを**まずスクロールや拡大として解釈しようとする**。
JS で `preventDefault()` すれば止められるが、それはイベントが来てからの話で、
ブラウザは「本当に来るのか」を待つあいだ描画を遅らせる（＝反応が鈍る）。

`touch-action` は CSS で先に宣言しておく仕組み。

```css
.pad-button { touch-action: none; }        /* この上では何も解釈するな */
body        { touch-action: manipulation; } /* ダブルタップ拡大だけ止める */
```

`manipulation` は「スクロールと二本指拡大は許すが、ダブルタップ拡大はしない」。
本文で `none` にしないのは、見えづらいときに拡大する逃げ道を残すためである。

なお viewport の `user-scalable=no` は **iOS Safari が無視する**（アクセシビリティ上の
判断で、10 以降そうなっている）。拡大を止める本命は `touch-action` のほう。

## `inputmode="none"` — 画面キーボードだけを断る

xterm.js はキー入力を、画面の外に隠した `<textarea>` で受けている。
スマホではテキストエリアにフォーカスが載った瞬間にソフトキーボードが出るので、
そのままだと画面の下半分が消える。

`inputmode` は「この入力欄にはどんなキーボードが向いているか」をブラウザに伝える属性
（`numeric` なら数字キーボード、など）。`none` はその中でも特別で、
**画面キーボードを出すな**という意味になる。

大事なのは、これが止めるのは*画面*キーボードだけだという点。外付けや内蔵の
物理キーボードからの入力はそのまま届く。だからタッチ対応のノート PC で
ボタンを出しても、キーボードで遊ぶ道は塞がらない。

## `pointer: coarse` — 指で操作する端末か

```js
window.matchMedia("(pointer: coarse)").matches
```

`pointer` メディア特性は、その端末の主なポインタがどれだけ細かく狙えるかを表す。
マウスなら `fine`、指なら `coarse`、ポインタが無ければ `none`。

これは「タッチ画面が付いているか」ではなく「主にどれで操作するか」なので、
タッチ対応のノート PC では `fine` になりうる。当てにしすぎない前提で、
実際に指で触られたかどうか（`pointerType`）と組み合わせている。

## `setTimeout` と `setInterval` を分けて持っている理由

```js
delayTimer = setTimeout(() => {
  repeatTimer = setInterval(() => press(button.id), REPEAT_INTERVAL_MS);
}, REPEAT_DELAY_MS);
```

初回だけ間を空け、そのあと一定間隔で繰り返す。変数を 2 つに分けているのは、
止めるときにどちらが動いているか分からないため。片方の変数に両方を入れて
使い回すこともできる（仕様上 `clearTimeout` と `clearInterval` は同じ表を引く）が、
読む人がそれを知っている前提のコードになる。

`stop()` を `pointerup` だけでなく `visibilitychange` や `blur` でも呼んでいるのは、
押したまま着信や画面切り替えが起きると `pointerup` が来ないことがあるためである。
止め忘れると、戻ってきた瞬間にミノが端まで走る。
