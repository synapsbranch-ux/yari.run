# yari.run clickable prototype (source)

This folder is the source of the yari.run design prototype: 48 artboards on one canvas.

- `project/canvas.json`: the canvas index (artboard frames, pages, the start view).
- `project/*.dc.html`: one file per artboard. Each is self-contained and uses the `.dc.html` format.
- `spec/`: the client brief, the build spec (tokens, components, data, flows) and the format notes the artboards were written against.

## Where to view it

**Standalone file (recommended): `yari-prototype.html`.** One self-contained HTML file with all 48 screens, the runtime and the fonts embedded. Download it and double-click to open it in any modern browser. No server, no install, works offline.

- Start on the index and pick one of the four flows, or press `/` to search all screens.
- Every link, button and form between screens works. `Back`, `Index` and `Screens` are in the top bar.
- On GitHub, use "Download raw file" (GitHub shows HTML as source, it does not run it).

The same prototype also runs on the design canvas:
https://claude.ai/artifact/WY53fSvprhVyn74G7TELFi

The loose `project/*.dc.html` files are the editable source. They need the canvas runtime and won't render on their own; use the standalone file or the canvas.

## Status

- Built from the brief, not reviewed by a second pass. Expect rough spots.
- Some copy and data are placeholders chosen for the prototype (for example the mango-shop data set).
