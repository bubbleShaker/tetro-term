// ブラウザ側のドライバの、JS が受け持つ半分。
//
// 役割は5つだけ:
//   1. xterm.js のターミナルを1つ用意し、Go が言ってきた大きさに合わせる
//   2. 時間を測って Go の tick を呼ぶ（requestAnimationFrame）
//   3. 端末が受け取ったバイト列を Go の data へそのまま渡す
//   4. Go から渡されたタッチボタンを並べ、押されたら Go の press へ id を返す
//   5. 端末にフォーカスを持たせる（タッチ端末を除く。後述）
//
// **ゲームのルールはここに一切無い**（docs/adr/0001）。キーが何を意味するかすら
// 知らない——矢印キーのバイト列を操作に読み替えるのは Go 側の internal/input で、
// CLI 版とまったく同じコードが通る。ここに「左キーなら…」と書き始めたら、
// その時点で CLI 版との一致は構造ではなく注意力の問題に変わる。
//
// タッチボタンも同じ扱いである。ボタンの一覧・文字・どちらの親指に置くかは
// すべて Go から渡ってきて、この JS が持つのは「押されたら id をそのまま返す」
// だけ。**ここでキーのバイト列（"\x1b[D" など）を組み立てないこと**——それを
// 始めた瞬間、キーの綴りという internal/input だけの持ち物がこちら側にも増える。

import { Terminal } from "./vendor/xterm.mjs";

const statusEl = document.getElementById("status");
const stageEl = document.getElementById("stage");
const terminalEl = document.getElementById("terminal");
const padEl = document.getElementById("pad");

// 操作説明はタッチ端末では引っ込めるが、GAME OVER や失敗の知らせは出したままにする。
// 両者が同じ 1 行を取り合うので、「いま出ているのがどちらか」を覚えておく。
let keyHelp = "";
let statusShowsHelp = false;

function showKeyHelp() {
  statusShowsHelp = true;
  // タッチ端末にキーの説明を出しても読む意味がない（そこに z キーは無い）。
  statusEl.textContent = touchMode ? "" : keyHelp;
}

function showMessage(text) {
  statusShowsHelp = false;
  statusEl.textContent = text;
}

function fail(message, err) {
  console.error(err);
  showMessage(`${message}: ${err?.message ?? err}`);
  statusEl.classList.add("error");
}

const term = new Terminal({
  convertEol: false, // 改行の扱いは Go 側に任せる（CRLF を Go が送る）
  cursorBlink: false,
  cursorStyle: "block",
  fontSize: 14,
  fontFamily: 'ui-monospace, "SF Mono", Menlo, Consolas, monospace',
  theme: {
    background: "#0d1017",
    foreground: "#c8d0e0",
  },
});

term.open(terminalEl);

// 端末を画面に収める。
//
// 桁数・行数は盤面に合わせて固定なので（Go の render.Size が決める）、収めるほうは
// 拡縮で行う。画面の広さに合わせて桁数を変える FitAddon は、盤面が固定である以上
// 逆向きの道具になったので使っていない。
//
// xterm の fontSize は触らず、要素ごと CSS の transform で拡縮する。fontSize を
// 計算する方式はセル幅と行高の丸めを自分で当てにいくことになり、1 桁はみ出す事故が
// 起きやすい（実寸は xterm の私有 API を覗かないと正確に取れない）。
// transform は文字を再ラスタライズするのでぼやけない。
function fitScale() {
  // offsetWidth / offsetHeight は transform の影響を受けない＝拡縮前の実寸が取れる。
  const naturalWidth = terminalEl.offsetWidth;
  const naturalHeight = terminalEl.offsetHeight;
  if (!naturalWidth || !naturalHeight) return;

  const scale = Math.min(
    stageEl.clientWidth / naturalWidth,
    stageEl.clientHeight / naturalHeight,
  );
  terminalEl.style.transform = `scale(${scale})`;
}

// 入れ物と端末の**両方**を見る。
// 入れ物は窓の大きさが変われば変わり、端末はフォントが確定して 1 文字の実寸が
// 決まり直せば変わる。片方しか見ていないと、もう片方が動いたときに倍率が古いまま残る。
// transform は行の組み直しを起こさないので、この観測が自分自身を呼び戻すことはない。
const observer = new ResizeObserver(fitScale);
observer.observe(stageEl);
observer.observe(terminalEl);

// 「タッチ端末か」を一発で当てる手段は無い。タッチ対応のノート PC もあれば、
// 外付けキーボードを繋いだタブレットもある。なので**判定と実績の両方**を使う:
// 指のような粗いポインタを持つ端末では最初から出し、そうでなくても実際に指で
// 触られたらその場で出す。判定だけだと「出ないと遊べない」側へ外れうるし、
// 実績だけだと最初の 1 タップが操作にならず、開いた人には壊れて見える。
//
// 誤って出したときの損は「使わないボタンが並ぶ」だけなので、外すなら出る方へ外す。
let touchMode = false;

// 最後に触ったのが指かどうか。フォーカスの戻し方をこれで変える（→ start）。
let lastPointerType = "";

function enableTouchMode() {
  if (touchMode) return;
  touchMode = true;
  document.body.classList.add("touch");

  // **ソフトキーボードを出させない**。xterm のフォーカス先は隠しテキストエリアで、
  // スマホではそこにフォーカスが載った瞬間に画面の下半分がキーボードに占領される。
  // inputmode="none" は「文字入力の口ではあるが画面キーボードは要らない」という
  // 指定で、外付けキーボードからの入力は通る——タッチ対応 PC でキーが死なない
  // のはこのためである。
  //
  // **付けるだけでは足りない**。粗いポインタとして検出されなかった端末では、
  // ここへ来るまでに start() が既にフォーカスを載せている。inputmode は
  // 「次にフォーカスされたとき」の話なので、もう出ているキーボードは引っ込まない。
  // 一度外して、載せ直させない。
  const textarea = terminalEl.querySelector("textarea");
  if (textarea) {
    textarea.inputMode = "none";
    textarea.blur();
  }

  if (statusShowsHelp) showKeyHelp();

  // ボタンが出た分だけ端末に使える高さが減る（→ fitScale）。
  // ResizeObserver も拾うが、そちらは次のフレームなので、ここで先に合わせておく。
  fitScale();
}

document.addEventListener(
  "pointerdown",
  (e) => {
    lastPointerType = e.pointerType;
    if (e.pointerType === "touch") enableTouchMode();
  },
  // ボタン側が pointerdown を preventDefault するので、捕捉相では必ず先に通る。
  { capture: true },
);

if (window.matchMedia?.("(pointer: coarse)").matches) enableTouchMode();

// Go 側から呼ばれる窓口。ここに生えているものだけが Go から見える。
globalThis.tetroTerm = {
  write: (s) => term.write(s),
  ready: (handlers) => start(handlers),
  gameOver: () => {
    // M2 にリスタートは無い（M7 / #8）。行き先はリロードしかないので、そう言う。
    showMessage("GAME OVER — リロードでもう一度遊べます");
    // 文章の行数が変われば端末に使える高さも変わる（→ start の順序についてのコメント）。
    fitScale();
  },
};

// 押しっぱなしで繰り返すときの間合い。
//
// CLI 版では OS のキーリピートがやっていることを、タッチでは誰もやってくれないので
// ここで作る。**これはゲームのルールではない**——「左へ 1 マス」を何回送るかという
// 入力装置の都合であり、だからコアではなくドライバのこちら側にある。
// どのボタンが繰り返してよいかを決めるのは Go（button.repeat）で、ここは間合いだけを持つ。
//
// 初回だけ間を空けるのは、1 マスだけ動かしたいときに 2 マス目が出ないようにするため。
const REPEAT_DELAY_MS = 200;
const REPEAT_INTERVAL_MS = 60;

// padButton は Go から来たボタン 1 つ分を DOM にする。press は Go の関数。
function padButton(button, press) {
  const el = document.createElement("button");
  el.type = "button";
  el.className = "pad-button";
  el.textContent = button.label;
  // 読み上げ用の説明も Go から来る。ここで日本語を書くと、操作を変えたときに
  // 説明だけ古くなる——操作説明の一行を Go から配っているのと同じ理由である。
  el.setAttribute("aria-label", button.what);

  let delayTimer = 0;
  let repeatTimer = 0;

  function stop() {
    clearTimeout(delayTimer);
    clearInterval(repeatTimer);
    delayTimer = 0;
    repeatTimer = 0;
    el.classList.remove("pressed");
  }

  el.addEventListener("pointerdown", (e) => {
    // 既定の動作を止める。通してしまうと、指の滑りがページのスクロールになり、
    // 離した瞬間に端末からフォーカスが飛び、続けて 2 回叩けば拡大になる。
    e.preventDefault();

    // 指を捕まえる。ボタンの外へ滑っても pointerup がこの要素に届くので、
    // 「離したのに動き続ける」が起きない。押しながら指を動かすのはよくある。
    el.setPointerCapture(e.pointerId);

    el.classList.add("pressed");
    // 押した瞬間に 1 回。溜めを待たせない。
    press(button.id);
    if (!button.repeat) return;

    delayTimer = setTimeout(() => {
      repeatTimer = setInterval(() => press(button.id), REPEAT_INTERVAL_MS);
    }, REPEAT_DELAY_MS);
  });

  for (const type of ["pointerup", "pointercancel", "pointerleave"]) {
    el.addEventListener(type, stop);
  }

  // 止め方はボタン 1 つの都合ではないので（→ buildPad）、外へ渡す。
  return { el, stop };
}

// buildPad は Go から来たボタン列を、左右の親指ごとにまとめて並べる。
//
// どちらの親指かは Go が side として付けてくる。こちらは並びを作るだけで、
// 「left が画面の左」以上のことは決めていない。並ぶ順は Go が返した順である。
function buildPad(buttons, press) {
  const sides = new Map();
  const stops = [];

  for (const button of buttons) {
    let side = sides.get(button.side);
    if (!side) {
      side = document.createElement("div");
      side.className = "pad-side";
      sides.set(button.side, side);
      padEl.append(side);
    }
    const { el, stop } = padButton(button, press);
    stops.push(stop);
    side.append(el);
  }

  // 押したままタブを離れる・着信で画面が変わると、pointerup が来ないことがある。
  // 止め忘れると、戻ってきた瞬間にミノが端まで走る。
  //
  // **ボタンごとではなく、ここで一度だけ引っ掛ける**。「全部止める」は個々の
  // ボタンの都合ではなく列全体の話であり、ボタンごとに登録すると同じ大域リスナが
  // ボタンの数だけ増える（増えた分は誰も外さない）。
  const stopAll = () => stops.forEach((stop) => stop());
  window.addEventListener("blur", stopAll);
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) stopAll();
  });
}

// start は Go の準備が終わったときに呼ばれる。tick・data・press はどれも Go の関数。
//
// 窓口を受け取るのと「準備ができた」を知るのが同じ 1 回なので、準備前に tick を
// 呼んでしまう順序を気にしなくてよい。
function start({ cols, rows, help, buttons, tick, data, press }) {
  // フレームぴったりの大きさにする。この値は Go の render.Size から来ており、
  // こちら側は 22 という数字を知らない。
  term.resize(cols, rows);

  // **拡縮より先に、縦を取り合うものを全部確定させる**。狭い画面ではこの一行が
  // 2 行にも 3 行にも折り返し、ボタン列も出れば出た分だけ端末に使える高さが減る。
  // 先に測ってしまうと、増えた分だけ端末が縦にはみ出す。
  // 操作説明も Go から来る。キーの割り当てを持っているのは internal/input だけで、
  // ここはそれを映すだけ。JS が独自に書くと、割り当てを変えたとき説明だけ古くなる。
  document.body.classList.add("ready");
  keyHelp = help;
  showKeyHelp();
  buildPad(buttons, press);

  fitScale();

  // 端末が受け取ったバイト列をそのまま Go へ。ここで意味を与えない。
  term.onData(data);

  // フォーカスが端末から外れると、矢印キーはページのスクロールに戻ってしまう。
  // 端末が持っている間は xterm が飲み込んでくれるので、持たせておく。
  // pointerdown ではなく click で拾うのは、フォーカスの既定の移動が先に起きてから
  // 戻したいためである（先に戻すと、そのあと body へ持っていかれる）。
  //
  // **指で触られたときだけは戻さない**。xterm のフォーカス先はテキストエリアなので、
  // スマホでそこにフォーカスを載せるとソフトキーボードが出てくる。inputmode="none"
  // でも塞いでいるが（→ enableTouchMode）、あれが効かないブラウザに備えて二重にする。
  // 指で遊んでいる間はキー入力が要らないので、戻さなくても失うものが無い。
  if (!touchMode) term.focus();
  document.addEventListener("click", () => {
    if (lastPointerType !== "touch") term.focus();
  });

  // ここが時間の出どころになる。Go 側に時計は無く、渡されたミリ秒だけを信じる。
  //
  // requestAnimationFrame を使うのは、背面タブでこれが止まるからである。setTimeout は
  // 1 秒に 1 回まで絞られるだけで止まらず、その間に溜まった時間をコアへ渡すと
  // 戻ってきた瞬間にミノが何段も落ちる（Go 側の clock.go も上限で押さえている）。
  let last = performance.now();
  function step(now) {
    tick(now - last);
    last = now;
    requestAnimationFrame(step);
  }
  requestAnimationFrame(step);
}

async function boot() {
  if (typeof globalThis.Go !== "function") {
    fail("起動できません", new Error("wasm_exec.js が読み込まれていません"));
    return;
  }

  const go = new globalThis.Go();

  // instantiateStreaming ではなく arrayBuffer 経由にしている。
  // 前者は Content-Type が application/wasm でないと失敗するため、
  // 手元の簡易サーバーでも動くようにこちらを使う。
  let instance;
  try {
    const res = await fetch("./main.wasm");
    if (!res.ok) throw new Error(`main.wasm の取得に失敗 (HTTP ${res.status})`);
    const bytes = await res.arrayBuffer();
    ({ instance } = await WebAssembly.instantiate(bytes, go.importObject));
  } catch (err) {
    fail("WebAssembly の読み込みに失敗しました", err);
    return;
  }

  // run() は Go の main が返るまで解決しない。main は待機し続ける設計なので、
  // ここで解決したらそれは異常終了を意味する。
  go.run(instance).then(
    () => fail("Go の実行が終了しました", new Error("main が予期せず return した")),
    (err) => fail("Go の実行が中断されました", err),
  );
}

boot();
