# yari.run clickable prototype (source)

This folder is the source of the yari.run design prototype: 48 artboards on one canvas.

- `project/canvas.json`: the canvas index (artboard frames, pages, the start view).
- `project/*.dc.html`: one file per artboard. Each is self-contained and uses the `.dc.html` format.
- `spec/`: the client brief, the build spec (tokens, components, data, flows) and the format notes the artboards were written against.

## Where to view it

The clickable version runs on the canvas, not from this repo:
https://claude.ai/artifact/WY53fSvprhVyn74G7TELFi

Open the "Start" page and press Play on the index board to walk the four flows.

The files here are the source. They won't render as clickable pages when opened directly from GitHub, because they depend on the canvas runtime (`support.js`) that the artifact host provides.

## Status

- Built from the brief, not reviewed by a second pass. Expect rough spots.
- Some copy and data are placeholders chosen for the prototype (for example the mango-shop data set).
