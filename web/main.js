// ブラウザ側の起動処理。
//
// 役割は2つだけ:
//   1. xterm.js のターミナルを1つ用意する
//   2. Go の WASM を起動し、Go から呼べる窓口 (globalThis.tetroTerm) を渡す
//
// ゲームのロジックはすべて Go 側にあり、この JS はそれを映す画面でしかない
// （docs/adr/0001-ansi-frame-as-the-only-boundary.md）。

import { Terminal } from "./vendor/xterm.mjs";
import { FitAddon } from "./vendor/addon-fit.mjs";

const statusEl = document.getElementById("status");

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

// FitAddon: コンテナの大きさから端末の桁数・行数を割り出してくれるアドオン。
// 端末の桁数はゲームの描画幅に直結するので、画面サイズ対応はこれに任せる。
const fitAddon = new FitAddon();
term.loadAddon(fitAddon);
term.open(document.getElementById("terminal"));
fitAddon.fit();

new ResizeObserver(() => {
  try {
    fitAddon.fit();
  } catch (err) {
    console.warn("fit に失敗", err);
  }
}).observe(document.getElementById("terminal"));

// Go 側から呼ばれる窓口。ここに生えているものだけが Go から見える。
globalThis.tetroTerm = {
  write: (s) => term.write(s),
  ready: () => {
    document.body.classList.add("ready");
    statusEl.textContent = "M0: Go → WASM → xterm.js の経路が通っています";
  },
};

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
