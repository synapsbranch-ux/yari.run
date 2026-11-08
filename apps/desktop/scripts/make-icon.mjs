// Generates src-tauri/app-icon.png: a neutral placeholder (dark rounded square, light "y").
// Pure Node, no dependencies. Replace the output with real artwork when a brand exists,
// then run `pnpm --filter desktop icons` (or `pnpm tauri icon <png>`) to derive the icon set.
import { writeFileSync } from 'node:fs';
import { deflateSync } from 'node:zlib';
import { fileURLToPath } from 'node:url';

const SIZE = 1024;
const SS = 3; // supersampling per axis
const BG = [24, 24, 27];
const FG = [244, 244, 245];

// Distance from point to a segment, in unit coordinates.
function segDist(px, py, ax, ay, bx, by) {
  const dx = bx - ax;
  const dy = by - ay;
  const t = Math.max(0, Math.min(1, ((px - ax) * dx + (py - ay) * dy) / (dx * dx + dy * dy)));
  return Math.hypot(px - (ax + t * dx), py - (ay + t * dy));
}

function insideRoundedSquare(x, y, r) {
  const cx = Math.min(Math.max(x, r), 1 - r);
  const cy = Math.min(Math.max(y, r), 1 - r);
  return Math.hypot(x - cx, y - cy) <= r;
}

function insideY(x, y) {
  const w = 0.055;
  return (
    segDist(x, y, 0.3, 0.3, 0.5, 0.58) <= w ||
    segDist(x, y, 0.7, 0.3, 0.38, 0.78) <= w
  );
}

const raw = Buffer.alloc((SIZE * 4 + 1) * SIZE);
for (let py = 0; py < SIZE; py++) {
  const row = py * (SIZE * 4 + 1);
  raw[row] = 0; // filter: none
  for (let px = 0; px < SIZE; px++) {
    let a = 0;
    let fg = 0;
    for (let sy = 0; sy < SS; sy++) {
      for (let sx = 0; sx < SS; sx++) {
        const x = (px + (sx + 0.5) / SS) / SIZE;
        const y = (py + (sy + 0.5) / SS) / SIZE;
        if (insideRoundedSquare(x, y, 0.22)) {
          a++;
          if (insideY(x, y)) fg++;
        }
      }
    }
    const n = SS * SS;
    const o = row + 1 + px * 4;
    const mix = a === 0 ? 0 : fg / a;
    raw[o] = Math.round(BG[0] + (FG[0] - BG[0]) * mix);
    raw[o + 1] = Math.round(BG[1] + (FG[1] - BG[1]) * mix);
    raw[o + 2] = Math.round(BG[2] + (FG[2] - BG[2]) * mix);
    raw[o + 3] = Math.round((255 * a) / n);
  }
}

const crcTable = new Uint32Array(256).map((_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});
function crc32(buf) {
  let c = 0xffffffff;
  for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}
function chunk(type, data) {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const td = Buffer.concat([Buffer.from(type, 'ascii'), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(td));
  return Buffer.concat([len, td, crc]);
}
const ihdr = Buffer.alloc(13);
ihdr.writeUInt32BE(SIZE, 0);
ihdr.writeUInt32BE(SIZE, 4);
ihdr[8] = 8; // bit depth
ihdr[9] = 6; // RGBA
const png = Buffer.concat([
  Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
  chunk('IHDR', ihdr),
  chunk('IDAT', deflateSync(raw, { level: 9 })),
  chunk('IEND', Buffer.alloc(0)),
]);
const out = fileURLToPath(new URL('../src-tauri/app-icon.png', import.meta.url));
writeFileSync(out, png);
console.log(`wrote ${out} (${png.length} bytes)`);
