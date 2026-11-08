# .dc.html authoring format (condensed from the canvas type's format reference)

Each artboard is one self-contained HTML file the design editor runs:

```html
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Hello</title>
<script src="./support.js"></script>
</head>
<body>
<x-dc>
<helmet>
<style>body{margin:0}</style>
</helmet>
<div style="padding: 32px">
  <h1>{{title}}</h1>
  <sc-for list="{{items}}" as="item" hint-placeholder-count="3">
    <div>{{item.label}}</div>
  </sc-for>
</div>
</x-dc>
<script type="text/x-dc" data-dc-script data-props='{"$preview":{"width":1440,"height":900}}'>
class Component extends DCLogic {
  renderVals() {
    return { title: 'Hello', items: [{ label: 'One' }] };
  }
}
</script>
</body>
</html>
```

- Keep `<script src="./support.js"></script>` EXACTLY. Close every non-void element; quote every attribute.
- Holes `{{ path }}` are dotted lookups only (`{{ user.name }}`, `{{ $index }}`, literals `{{ true }}`). Never expressions (`{{ a + b }}`, `{{ !x }}`, `{{ fn() }}` fail silently). Compute in `renderVals()`.
- Attributes: `x="literal"` → string; `x="{{ path }}"` → the raw value (number, function); `x="a {{p}} b"` → interpolated string. `class`/`for` map automatically.
- Events: whole-value camelCase attrs: `onClick="{{ pick }}"`, `onInput="{{ onType }}"`, `onChange`, `onMouseEnter`, `onMouseLeave`, `onFocus`, `onBlur`, `onKeyDown` (on an element, not global). Handlers come from `renderVals()`. Per-item: `items: xs.map(x => ({...x, pick: () => this.setState({picked: x.id})}))` then `onClick="{{ item.pick }}"` in the loop. For inputs: `value="{{ typed }}" onInput="{{ onType }}"` with `onType: (e) => this.setState({typed: e.target.value})`.
- `<sc-if value="{{ cond }}" hint-placeholder-val="{{ true }}">…</sc-if>` (no else: use two flags). `<sc-for list="{{ items }}" as="item" hint-placeholder-count="3">` with `{{ item.x }}`, `{{ $index }}`.
- Links between artboards: `<a href="Cart.dc.html">View cart</a>` (path relative to this artboard). Style the `<a>` itself as the button; never nest a `<button>` in an `<a>`. `href="#id"` scrolls within the board.
- Each artboard keeps its own `state`; a multi-step flow inside one board uses a `state` field, `renderVals()` returns one boolean per step, each step in its own `<sc-if>`.
- Conditional styling in a loop: precompute per item (`style`, `selected`) in `renderVals()` and bind `style="{{ item.style }}"`. Static theme colors stay literal inline.
- Logic: classic JS only, `class Component extends DCLogic`, no import/export. `this.props`, `this.state`/`setState`, lifecycle (`componentDidMount`, `componentWillUnmount`), no `render()`. Initialize state in `constructor(props){ super(props); this.state = {...}; }`.
- `data-props` (single-quoted JSON on the script tag) declares tweaks: `{"step":{"editor":"enum","options":["a","b"],"default":"a"},"$preview":{"width":1440,"height":900}}`. In JSON inside the single-quoted attribute: write `&amp;` for &, `&#39;` for a single quote. Few levers only; never copy text.
- All UI is `<x-dc>` markup, never script-built (no innerHTML/appendChild). No global keydown handlers. No iframe/object/embed. Icons: inline stroke SVG. No emoji.
- A PAGE (desktop app screen) is fluid: no fixed px width on the root (width 100%), px heights allowed; wide content scrolls inside an overflow-x:auto box. Phone frames: fixed 390×844 root.
- Diagrams/canvases: build nodes from separate positioned elements (not one big svg). One aria-hidden svg for edge lines behind nodes is fine.
- Accessibility: real `<button type="button">`, `<a href>`, `<input>` + `<label>`; `aria-label` on icon-only buttons; decorative SVG `aria-hidden="true"`. Text contrast ≥ 4.5:1.
