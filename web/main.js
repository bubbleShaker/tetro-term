// ブラウザ側のドライバの、JS が受け持つ半分。
//
// 役割は4つだけ:
//   1. xterm.js のターミナルを1つ用意し、Go が言ってきた大きさに合わせる
//   2. 時間を測って Go の tick を呼ぶ（requestAnimationFrame）
//   3. 端末が受け取ったバイト列を Go の data へそのまま渡す
//   4. 端末にフォーカスを持たせ続ける
//
// **ゲームのルールはここに一切無い**（docs/adr/0001）。キーが何を意味するかすら
// 知らない——矢印キーのバイト列を操作に読み替えるのは Go 側の internal/input で、
// CLI 版とまったく同じコードが通る。ここに「左キーなら…」と書き始めたら、
// その時点で CLI 版との一致は構造ではなく注意力の問題に変わる。

import { Terminal } from "./vendor/xterm.mjs";

const statusEl = document.getElementById("status");
const stageEl = document.getElementById("stage");
const terminalEl = document.getElementById("terminal");

const KEY_HELP = "←→ 移動  ↑ 回転  z 反時計回り  ↓ ソフトドロップ";

function fail(message, err) {
  console.error(err);
  statusEl.textContent = `${message}: ${err?.message ?? err}`;
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

new ResizeObserver(fitScale).observe(stageEl);

// Go 側から呼ばれる窓口。ここに生えているものだけが Go から見える。
globalThis.tetroTerm = {
  write: (s) => term.write(s),
  ready: (handlers) => start(handlers),
  gameOver: () => {
    // M2 にリスタートは無い（M7 / #8）。行き先はリロードしかないので、そう言う。
    statusEl.textContent = "GAME OVER — リロードでもう一度遊べます";
    // 文章の行数が変われば端末に使える高さも変わる（→ start の順序についてのコメント）。
    fitScale();
  },
};

// start は Go の準備が終わったときに呼ばれる。tick と data はどちらも Go の関数。
//
// 窓口を受け取るのと「準備ができた」を知るのが同じ 1 回なので、準備前に tick を
// 呼んでしまう順序を気にしなくてよい。
function start({ cols, rows, tick, data }) {
  // フレームぴったりの大きさにする。この値は Go の render.Size から来ており、
  // こちら側は 22 という数字を知らない。
  term.resize(cols, rows);

  // **拡縮より先に文章を確定させる**。狭い画面ではこの一行が 2 行にも 3 行にも折り返し、
  // その分だけ端末に使える高さが減る。先に測ってしまうと、増えた行数のぶんだけ
  // 端末が縦にはみ出す。
  document.body.classList.add("ready");
  statusEl.textContent = KEY_HELP;

  fitScale();

  // 端末が受け取ったバイト列をそのまま Go へ。ここで意味を与えない。
  term.onData(data);

  // フォーカスが端末から外れると、矢印キーはページのスクロールに戻ってしまう。
  // 端末が持っている間は xterm が飲み込んでくれるので、常に持たせておく。
  // pointerdown ではなく click で拾うのは、フォーカスの既定の移動が先に起きてから
  // 戻したいためである（先に戻すと、そのあと body へ持っていかれる）。
  term.focus();
  document.addEventListener("click", () => term.focus());

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
