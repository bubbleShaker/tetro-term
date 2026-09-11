// ブラウザ版のタッチ操作を、実際に指で触って確かめる。
//
// **なぜ Go のテストで足りないか**: Go 側が守れるのは「ボタンの一覧」と
// 「id から入力への読み替え」までで（internal/input/touch_test.go）、
// そこから先——ボタンが本当に DOM になり、指で押され、押しっぱなしで繰り返し、
// 離すと止まるか——は web/main.js の受け持ちになる。ここが壊れても PC の
// キーボードでは最後まで遊べてしまうので、気づくのはスマホを出したときになる。
//
// **CI には入れていない**。Go のリポジトリに node_modules とブラウザを
// 引き込むことになり、守りたいものの大きさに釣り合わない。
// 手元で回す道具として置いてある。
//
//   ./scripts/build-wasm.sh
//   npm i playwright && npx playwright install chromium   # 初回だけ
//   node scripts/check-touch.mjs
//
// 別のプロジェクトに入っている playwright を借りるときは、その node_modules を渡す:
//   PLAYWRIGHT_FROM=/path/to/node_modules node scripts/check-touch.mjs

import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import path from "node:path";

const webDir = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "web");

// ESM の import は NODE_PATH を見ないので、借り物は require で解決する。
function loadPlaywright() {
  const from = process.env.PLAYWRIGHT_FROM;
  if (from) {
    return createRequire(path.join(from, "borrowed.js"))("playwright");
  }
  return createRequire(import.meta.url)("playwright");
}

let chromium;
try {
  ({ chromium } = loadPlaywright());
} catch {
  console.error(
    "playwright が見つからない。`npm i playwright && npx playwright install chromium` を実行するか、\n" +
      "既に入っている node_modules を PLAYWRIGHT_FROM で渡してほしい。",
  );
  process.exit(2);
}

// --- web/ を配る -----------------------------------------------------------
const types = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".wasm": "application/wasm",
};

const server = createServer(async (req, res) => {
  const rel = decodeURIComponent(new URL(req.url, "http://x").pathname);
  const file = path.join(webDir, rel === "/" ? "index.html" : rel);
  if (!file.startsWith(webDir)) {
    res.writeHead(403).end();
    return;
  }
  try {
    const body = await readFile(file);
    res.writeHead(200, { "content-type": types[path.extname(file)] ?? "application/octet-stream" });
    res.end(body);
  } catch {
    res.writeHead(404).end();
  }
});
await new Promise((done) => server.listen(0, "127.0.0.1", done));
const origin = `http://127.0.0.1:${server.address().port}`;

// --- 検証 ------------------------------------------------------------------
const results = [];
function check(name, ok, detail = "") {
  results.push({ name, ok });
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? "  — " + detail : ""}`);
}

const browser = await chromium.launch();
const context = await browser.newContext({
  hasTouch: true,
  isMobile: true,
  viewport: { width: 390, height: 844 }, // 手のひらに収まる縦画面
  deviceScaleFactor: 3,
});

await context.addInitScript(() => {
  // requestAnimationFrame が回らない環境（WSL の headless など）でも tick が
  // 進むようにする。**検証のための細工**であり、本物のブラウザには要らない。
  window.requestAnimationFrame = (cb) => setTimeout(() => cb(performance.now()), 16);

  // Go へ渡る press と、Go から来るフレームを横取りして控える。
  // 窓口の代入そのものを捕まえるので、main.js に検証用の細工を足さずに済む。
  let real;
  Object.defineProperty(window, "tetroTerm", {
    configurable: true,
    get: () => real,
    set: (v) => {
      real = v;
      window.__presses = [];
      window.__frames = [];
      const origWrite = v.write;
      v.write = (s) => {
        window.__frames.push(s);
        return origWrite.call(v, s);
      };
      const origReady = v.ready;
      v.ready = (handlers) => {
        const origPress = handlers.press;
        handlers.press = (id) => {
          window.__presses.push(id);
          return origPress(id);
        };
        return origReady.call(v, handlers);
      };
    },
  });
});

const page = await context.newPage();
page.on("pageerror", (e) => check("ページで例外が出ていない", false, e.message));
await page.goto(origin, { waitUntil: "load" });
await page.waitForSelector(".pad-button", { timeout: 30000 });

const cdp = await context.newCDPSession(page);
// Playwright の tap() は押して離すまでが一息なので、押しっぱなしを作れない。
// 押す・離すを別々に送るために CDP を直に叩く。
async function touch(type, x = 0, y = 0) {
  await cdp.send("Input.dispatchTouchEvent", {
    type,
    touchPoints: type === "touchEnd" ? [] : [{ x, y }],
  });
}
async function centerOf(label) {
  const box = await page.locator(`.pad-button[aria-label="${label}"]`).boundingBox();
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}
async function hold(label, ms) {
  const { x, y } = await centerOf(label);
  await page.evaluate(() => (window.__presses.length = 0));
  await touch("touchStart", x, y);
  await page.waitForTimeout(ms);
  await touch("touchEnd");
  return page.evaluate(() => window.__presses.slice());
}

// ボタンが Go の表どおりに並んでいること
const labels = await page.$$eval(".pad-button", (els) =>
  els.map((e) => `${e.textContent}/${e.getAttribute("aria-label")}`),
);
check("Go が配ったボタンが並ぶ", labels.length === 5, labels.join(" "));

const sides = await page.$$eval(".pad-side", (els) => els.map((e) => e.childElementCount));
check("左右の親指に分かれている", JSON.stringify(sides) === "[3,2]", JSON.stringify(sides));

check(
  "タッチ端末としてボタン列が出ている",
  await page.evaluate(
    () =>
      document.body.classList.contains("touch") &&
      getComputedStyle(document.getElementById("pad")).display === "flex",
  ),
);

// ソフトキーボードが画面を食い潰さないこと。守り方は 2 つあり、両方見る。
check(
  "テキストエリアが画面キーボードを呼ばない",
  await page.evaluate(() => document.querySelector("#terminal textarea")?.inputMode === "none"),
);
check(
  "端末にフォーカスが載っていない",
  await page.evaluate(() => document.activeElement !== document.querySelector("#terminal textarea")),
);

// 押す・押しっぱなし・離す
const tapped = await hold("左へ移動", 50);
check("軽く叩くと 1 回だけ", tapped.length === 1 && tapped[0] === "left", JSON.stringify(tapped));

const held = await hold("左へ移動", 500);
// 押した瞬間に 1 回、200ms 待って以降 60ms ごと → 6 回前後
check(
  "押しっぱなしで繰り返す",
  held.length >= 5 && held.length <= 8 && held.every((id) => id === "left"),
  `500ms で ${held.length} 回`,
);

const rotated = await hold("時計回りに回転", 500);
check(
  "回転は押しっぱなしでも 1 回だけ",
  rotated.length === 1 && rotated[0] === "rotateCW",
  `${rotated.length} 回`,
);

await page.evaluate(() => (window.__presses.length = 0));
await page.waitForTimeout(300);
check(
  "離したあとは繰り返しが止まる",
  (await page.evaluate(() => window.__presses.length)) === 0,
);

// 押したままボタンの外へ指を滑らせて離す。捕まえ損ねると走り続ける。
{
  const { x, y } = await centerOf("左へ移動");
  await page.evaluate(() => (window.__presses.length = 0));
  await touch("touchStart", x, y);
  await page.waitForTimeout(300);
  await touch("touchMove", x + 200, y - 200);
  await touch("touchEnd");
  await page.evaluate(() => (window.__presses.length = 0));
  await page.waitForTimeout(300);
  check(
    "ボタンの外で指を離しても止まる",
    (await page.evaluate(() => window.__presses.length)) === 0,
  );
}

// ボタンが Go の game.Handle まで届いていること。
//
// レンダラはセルを「背景色を塗った空白」として出すので、文字ではなく背景色の
// 指定（ESC [ 48;5;N m）が何桁目から始まるかを見る。左へ寄せきったときと
// 右へ寄せきったときで列がずれれば、押した結果が盤面に出ている。
// 重力で落ちても列は変わらないので、落下と取り違えない。
async function leftmostPaintedColumn(label) {
  // ここまでで盤面は散々動かされている。素の状態から測り直す。
  await page.reload({ waitUntil: "load" });
  await page.waitForSelector(".pad-button");
  await page.waitForTimeout(200);

  const { x, y } = await centerOf(label);
  await touch("touchStart", x, y);
  await page.waitForTimeout(900); // 盤面の端まで寄るだけの繰り返し
  await touch("touchEnd");
  await page.waitForTimeout(100);

  const frame = await page.evaluate(() => window.__frames.at(-1));
  let leftmost = Infinity;
  for (const line of frame.split("\r\n")) {
    const re = /\x1b\[[0-9;]*[A-Za-z]/g;
    let col = 0;
    let last = 0;
    let m;
    while ((m = re.exec(line)) !== null) {
      col += m.index - last;
      last = re.lastIndex;
      if (m[0].includes("48;5;")) {
        leftmost = Math.min(leftmost, col);
        break;
      }
    }
  }
  return leftmost;
}

const atLeft = await leftmostPaintedColumn("左へ移動");
const atRight = await leftmostPaintedColumn("右へ移動");
check(
  "押した結果がゲームに届いている",
  Number.isFinite(atLeft) && Number.isFinite(atRight) && atLeft < atRight,
  `左寄せ ${atLeft} 桁目 / 右寄せ ${atRight} 桁目`,
);

await browser.close();
server.close();

const failed = results.filter((r) => !r.ok).length;
console.log(`\n${results.length - failed} / ${results.length} 通過`);
process.exit(failed ? 1 : 0);
