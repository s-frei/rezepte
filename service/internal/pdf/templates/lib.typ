// Shared by recipe.typ and book.typ. A custom template tree carries its own
// copy, so every tree stands alone.
//
// This is the design of the printed recipe: printing, downloading and the
// pass-on sheet's preview all render it. User text is placed only as strings
// (`text(v)`, `[#v]`), never evaluated, so markup in a title prints literally.

// The light design tokens of app.css.
#let ink = rgb("#2b2118")
#let muted = rgb("#756554")
#let primary = rgb("#b45309")
#let amount = rgb("#7c3d0a")
// Print-only colors, not app tokens. `rule` is a hairline dark enough
// (about a 35 % tint) that an office laser prints it solid rather than
// dithered into dots. `text-ink` is a neutral near-black for the small
// text (steps, ingredient rows, footer values): a driver prints R = G = B
// with black toner alone, so small Karla stays sharp on a color device.
// Display type and the structure rules keep the warm `ink`.
#let rule = rgb("#b3a28a")
#let text-ink = rgb("#1e1e1e")
#let display = "Literata"
#let sans = "Karla"
// The rule weights: the hairline, the structure rule under the card's
// title, and A4's ingredient rail, lighter than the structure so it never
// outweighs the title.
#let hairline = 0.5pt + rule
#let structure = 1.2pt + ink
#let rail-rule = 0.8pt + ink

// Ascender and descender of each face, in em.
#let metrics = ((sans): (0.917, 0.252), (display): (1.177, 0.308))

// CSS line boxes: `line-height: h` for font f, as Typst text edges, so a
// line is exactly h em tall and every gap is an explicit value.
#let lh(h, f: sans) = {
  let (a, d) = metrics.at(f)
  let half = (h - a - d) / 2
  (top-edge: (a + half) * 1em, bottom-edge: -(d + half) * 1em)
}

// Where the baseline of a `line-height: h` line of size `size` sits below
// the line's top.
#let baseline(h, size, f: sans) = lh(h, f: f).top-edge.em * size

#let caps(v, size: 7.5pt, weight: 600, tracking: 0.12em, h: 1.4) = text(
  size: size, weight: weight, tracking: tracking, fill: muted, ..lh(h), upper(v),
)

#let is-card(data) = data.options.paper == "a5"

// The sheet: a punching gutter on the left; `cue`, the gap from the content
// to the running head's baseline above it and to the foot hint's cap height
// below it, so the two lines mirror each other about 7 mm (A4) from the
// paper's edge, clear of a printer's unprintable rim, and the head stands
// about one and a half lines above the text; the footer band's QR size,
// the photo's height, and the type: text size and line height, the
// ingredient rows' size and cell padding, the step numbers, their
// right-aligned column and the gap after it, and the space between steps
// and between ingredient groups. `columns`
// is how many columns a list set above the steps takes; a step longer than
// `long` characters (its text and amounts) may break across pages, a
// shorter one, about 12 lines, never does.
#let geometry(card) = if card {
  (width: 120mm, height: 188mm, margin: (top: 11mm, right: 10mm, bottom: 11mm, left: 18mm),
    cue: 3.5mm, qr: 20mm, gap: 3mm, photo: 24mm,
    text: 10pt, lh: 1.4, list: 9.5pt, row: 1.3pt, number: 11pt, num-w: 5mm, num-gap: 2mm,
    step-gap: 4pt, group-gap: 6pt, columns: 2, long: 800)
} else {
  (width: 174mm, height: 269mm, margin: (top: 14mm, right: 14mm, bottom: 14mm, left: 22mm),
    cue: 4.5mm, qr: 22mm, gap: 6mm, photo: 60mm,
    text: 10.5pt, lh: 1.45, list: 10pt, row: 1.6pt, number: 14pt, num-w: 6mm, num-gap: 2.5mm,
    step-gap: 7pt, group-gap: 8pt, columns: 3, long: 640)
}

// The steps' measure on A4 when nothing stands beside them, and A4's
// ingredient rail beside the steps and the gap before it. A list that would
// run on more than `list-first` (about six rows) below the steps beside it
// goes above them instead, so the method has no hole under it.
#let measure-alone = 120mm
#let rail = 64mm
#let rail-gap = 9mm
#let list-first = 33mm
// The ingredient rows' quantity and unit columns: one pair per recipe, each
// as wide as its widest cell across every group, set as it prints (bold
// quantities, regular units), plus the space after it, so every group shares
// one vertical for the units and one for the names. Needs context.
#let qty-gap = 3pt
#let unit-gap = 4pt
#let ingredient-columns(r, g) = {
  let all = r.ingredientGroups.map(gr => gr.ingredients).flatten()
  let widest(weight, f) = calc.max(0pt, ..all.map(i => measure(text(size: g.list, weight: weight, f(i))).width))
  (
    qw: widest(700, i => if i.quantity == none { "" } else { i.quantity.display }) + qty-gap,
    uw: widest(400, i => if i.unit == none { "" } else { i.unit }) + unit-gap,
  )
}
// The column gap of a list set in columns.
#let column-gap = 7mm

// The host of a URL, without scheme and a leading "www.".
#let host(url) = url.replace(regex("^[a-zA-Z][a-zA-Z0-9+.-]*://"), "").replace(regex("^www\."), "").split("/").first()

// A one-line text: make(v), cut at its end with "…" to the longest start
// that fits max.
#let one-line(make, v, max) = context {
  let cs = v.clusters()
  let cut(n) = if n == cs.len() { v } else { cs.slice(0, n).join(default: "").trim() + "…" }
  let fits(n) = measure(make(cut(n))).width <= max
  // fits(lo) holds or lo is 0; fits(hi) does not.
  let (lo, hi) = (0, cs.len())
  if fits(hi) { lo = hi }
  while hi - lo > 1 {
    let mid = calc.quo(lo + hi, 2)
    if fits(mid) { lo = mid } else { hi = mid }
  }
  make(cut(lo))
}

// One line as wide as the region it is placed in, cut with "…".
#let fill-line(make, v) = layout(size => one-line(make, v, size.width))

// The recipe page p belongs to, as its `<recipe-page>` marker's value
// (page, recipe): the last one started on or before it, none before the
// first. The page header and the book's contents both ask it. Needs context.
#let recipe-at(p) = {
  let started = query(<recipe-page>).filter(m => m.value.page <= p)
  if started.len() > 0 { started.last().value }
}

// Page setup for a recipe or a book: paper, margins, fonts, the running
// title on a recipe's second page on, the hint at the foot of every page a
// recipe goes on from, and the folio "x / y" in a book from page 2.
#let setup(data, body) = {
  let card = is-card(data)
  let g = geometry(card)
  let book = data.options.bundle == "book"
  let labels = data.labels
  set page(
    paper: if card { "a5" } else { "a4" },
    margin: g.margin,
    header-ascent: g.cue,
    footer-descent: g.cue,
    header: context {
      let p = here().page()
      set text(size: 8pt, bottom-edge: "baseline")
      let at = recipe-at(p)
      let head = if at != none and at.page < p {
        let t = data.recipes.at(at.recipe).title + " (" + labels.continued + ")"
        [#metadata(t)<running-header>]
        fill-line(v => text(weight: 500, fill: muted, v), t)
      }
      let folio = if book and p > 1 {
        let f = str(counter(page).get().first()) + " / " + str(counter(page).final().first())
        [#metadata((page: p, text: f))<folio>]
        text(weight: 600, f)
      }
      grid(columns: (1fr, auto), column-gutter: 6mm, head, folio)
    },
    footer: context {
      let p = here().page()
      let at = recipe-at(p)
      if at == none { return }
      let end = query(<recipe-end>).find(m => m.value == at.recipe)
      if end != none and end.location().page() > p {
        [#metadata((recipe: at.recipe, page: p))<next-page>]
        align(right, text(size: 8pt, weight: 500, fill: muted, top-edge: "cap-height", labels.nextPage))
      }
    },
  )
  // Ragged right with no hyphens and greedy line breaks: Typst splits even
  // short words ("gele-sen"), so only ingredient names hyphenate, and
  // greedy breaks keep the right edge calm instead of pulling a word back.
  set text(
    font: sans, size: g.text, lang: data.locale, hyphenate: false,
    fill: ink, number-type: "lining", number-width: "tabular", ..lh(g.lh),
  )
  set par(leading: 0pt, spacing: 0pt, linebreaks: "simple", justify: false)
  set block(spacing: 0pt)
  show heading: it => it.body
  context [#metadata(counter(page).final().first())<page-count>]
  body
}

// The cover cropped to w × h around its focus, like `focusPosition`.
#let cover(p, w, h) = {
  let a = p.width / p.height
  let (iw, ih) = if a > w / h { (h * a, h) } else { (w, w / a) }
  let dx = -calc.min(calc.max(p.focus.x * iw - w / 2, 0pt), iw - w)
  let dy = -calc.min(calc.max(p.focus.y * ih - h / 2, 0pt), ih - h)
  box(width: w, height: h, clip: true, radius: 2pt, place(top + left, dx: dx, dy: dy, image(p.path, width: iw, height: ih)))
}

#let title(r, card, outlined) = {
  let t = text(r.title)
  [#metadata(r.title)<recipe-title>#metadata(t)<title-text>]
  // Styled by a show-set, so the contents list gets the bare title and the
  // heading's own default size never applies.
  show heading: set text(
    font: display, size: if card { 20pt } else { 30pt }, weight: if card { 600 } else { 500 },
    tracking: if card { -0.01em } else { -0.015em }, ..lh(if card { 1.1 } else { 1.05 }, f: display),
  )
  heading(level: 1, outlined: outlined, t)
}

#let section-heading(v) = block(below: 5pt, sticky: true,
  text(font: display, size: 12pt, weight: 600, ..lh(1.2, f: display), v))

// Marks where a column of recipe `ri` ends. In a block of its own: a bare
// tag takes the position of what follows it, the reserve's spacing included.
#let column-end(ri, label: <column-end>) = block(height: 0pt, [#metadata(ri)#label])

#let is-long(s, g) = {
  let n = s.segments.map(seg => seg.text.codepoints().len() + if seg.amount == none { 0 } else { seg.amount.codepoints().len() }).sum(default: 0)
  n > g.long
}

// One step of recipe `ri`. `sticky` keeps it on the page of the next one;
// `reserve` is the room the last step holds below itself for the footer
// (see `foot`), and only the last step gets one or ends the column.
#let step(s, ri, g, last: false, sticky: false, reserve: none) = {
  // The number's line box ends where the text's first line has its
  // baseline, so both share it.
  let n = text(
    font: display, size: g.number, weight: 600, fill: primary,
    top-edge: baseline(g.lh, g.text), bottom-edge: "descender", str(s.number),
  )
  let body = for seg in s.segments {
    if seg.kind == "ingredient" {
      // SemiBold flags the ingredient and the brown alone the amount, so a
      // step dense with references still reads as a sentence.
      [#metadata(seg.text)<ingredient-ref>]
      text(weight: 600, seg.text)
      if seg.amount != none {
        [~#box(text(size: g.list, fill: amount, "(" + seg.amount + ")"))]
      }
    } else { text(seg.text) }
  }
  let long = is-long(s, g)
  context {
    let at = here().position()
    [#metadata((recipe: ri, number: s.number, page: at.page, y: at.y.mm(), long: long))<step>]
  }
  block(breakable: long, sticky: sticky, below: if last { 0pt } else { g.step-gap }, {
    // The numbers hang right-aligned in their column, so "9" and "12"
    // share a right edge and the text never moves.
    grid(columns: (g.num-w, 1fr), column-gutter: g.num-gap, align(right, box(context [#metadata(measure(n).width.mm())<step-number>#n])),
      par(text(fill: text-ink, [#metadata(s.number)<step-text>#body])))
    if last { column-end(ri) }
    if reserve != none { v(reserve) }
  })
}

#let steps(r, ri, labels, g, card, reserve: none) = {
  if r.steps.len() == 0 { return }
  if not card { section-heading(labels.steps) }
  // The heading (sticky) and the first step stick to the second, so a page
  // never ends on a method's start; the second-to-last step sticks to the
  // last, which holds the footer's room, so the footer's page holds at
  // least two steps.
  let n = r.steps.len()
  for (k, s) in r.steps.enumerate() {
    let last = k == n - 1
    step(s, ri, g, last: last, sticky: not last and (k == 0 or k == n - 2), reserve: if last { reserve })
  }
}

#let group-heading(v) = {
  [#metadata(v)<group-heading>]
  block(below: 2.5pt, caps(v, weight: 700, tracking: 0.1em, h: 1.45))
}

// Text whose words wrap whole or hyphenate, but a word that hyphenation
// cannot fit in the column breaks between any two letters, so nothing runs
// out of its cell. `measure` clamps an overflowing line to the width it is
// given. Needs context.
#let fit(s, width) = s.split(" ").map(w => {
  if measure(w, width: width).width < width { w } else { w.clusters().join(sym.zws) }
}).join(" ")

// One ingredient group: quantity right-aligned, unit, name, in the recipe's
// columns (`g.qw`, `g.uw`, see `ingredient-columns`). Up to 12 rows it never
// splits; a longer one breaks and repeats its name with "(cont.)".
#let group(gr, id, labels, g) = {
  let rows = gr.ingredients.enumerate().map(((n, i)) => (
    {
      // The marker rides in the first row, so it lands where the group
      // starts, even when the group moves on to the next page.
      if n == 0 { context [#metadata((id: id, x: here().position().x.mm()))<group-start>] }
      [#metadata(i.display)<quantity>]
      if i.quantity != none { text(weight: 700, i.quantity.display) }
    },
    if i.unit != none { text(fill: muted, i.unit) },
    {
      // Names hyphenate to fit the narrow cell; their notes do not.
      set text(hyphenate: true)
      layout(size => {
        [#metadata(text(i.name))<ingredient-name>]
        let name = fit(i.name + if i.note != none { "," }, size.width)
        let body = if i.note == none { name } else { [#name #text(fill: muted, hyphenate: false, fit(i.note, size.width))] }
        let (width, height) = measure(body, width: size.width)
        let broken = name.contains(str(sym.zws))
        [#metadata((width: width.mm(), height: height.mm(), column: size.width.mm(), broken: broken))<name-size>]
        body
      })
    },
  )).flatten()
  set text(size: g.list, fill: text-ink, ..lh(1.3))
  let cells(header) = grid(
    columns: (g.qw, g.uw, 1fr), align: (right, left, left),
    inset: (x, _) => (y: g.row, right: (qty-gap, unit-gap, 0pt).at(x)),
    ..header, ..rows,
  )
  if gr.ingredients.len() <= 12 {
    block(breakable: false, {
      if gr.name != none { group-heading(gr.name) }
      cells(())
    })
  } else if gr.name == none {
    cells(())
  } else {
    let head = grid.header(grid.cell(colspan: 3, inset: 0pt, align: left, context {
      let start = query(<group-start>).find(m => m.value.id == id)
      let first = start == none or here().page() == start.location().page()
      group-heading(if first { gr.name } else { gr.name + " (" + labels.continued + ")" })
    }))
    cells((head,))
  }
}

// The groups, given as (id, group) pairs, stacked with the space between
// groups.
#let stack(part, labels, g) = for (n, (id, gr)) in part.enumerate() {
  block(below: if n == part.len() - 1 { 0pt } else { g.group-gap }, group(gr, id, labels, g))
}

// The groups over n columns of width w, balanced: at the lowest height at which a column-major fill takes n columns, each column
// as full as it gets. A group of up to 12 rows stays whole; a longer one may
// be cut once between rows, its second part repeating the heading with
// "(cont.)". Needs a context. Returns the columns, each a list of
// (id, group) pairs.
#let balance(parts, n, w, labels, g) = {
  let h(gr) = measure(block(width: w, group(gr, "", labels, g))).height
  let cont(gr) = if gr.name == none { none } else { gr.name + " (" + labels.continued + ")" }
  // Each part with its height, and for a long one every cut: the two
  // pieces and their heights.
  // ponytail: every row of a long group is a cut, each measured whole, so a
  // group of k rows costs about k² row measures; build only the cuts that
  // fit the current limit if lists grow far past today's 30 rows.
  let items = parts.map(((id, gr)) => {
    let k = gr.ingredients.len()
    let cuts = if k <= 12 { () } else {
      range(1, k).map(row => {
        let top = (name: gr.name, ingredients: gr.ingredients.slice(0, row))
        let rest = (name: cont(gr), ingredients: gr.ingredients.slice(row))
        (top: (id, top), top-h: h(top), rest: (id + "b", rest), rest-h: h(rest))
      })
    }
    (part: (id, gr), height: h(gr), cuts: cuts)
  })
  // Column-major: a part goes on the current column when it fits the limit
  // there (or the column is empty and no cut fits), else its longest
  // fitting top piece does and the rest starts the next column, else it
  // starts the next column whole. A long group, or its rest, taller than
  // the limit means the limit is too low (none): it could have been cut.
  let fill(limit) = {
    let cols = ((),)
    let used = 0pt
    for it in items {
      let empty = cols.last().len() == 0
      let base = if empty { 0pt } else { used + g.group-gap }
      let cut = it.cuts.rev().find(c => base + c.top-h <= limit)
      if base + it.height <= limit or (empty and cut == none) {
        cols.last().push(it.part)
        used = base + it.height
      } else if cut != none {
        cols.last().push(cut.top)
        cols.push((cut.rest,))
        used = cut.rest-h
      } else {
        cols.push((it.part,))
        used = it.height
      }
      if it.cuts.len() > 0 and used > limit { return none }
    }
    cols
  }
  // The lowest height that fits n columns, to a quarter point.
  let (lo, hi) = (0pt, items.map(it => it.height).sum() + g.group-gap * items.len())
  while hi - lo > 0.25pt {
    let mid = (lo + hi) / 2
    let cols = fill(mid)
    if cols != none and cols.len() <= n { hi = mid } else { lo = mid }
  }
  fill(hi)
}

// The list in columns above the steps, on the card and on A4 when it goes
// first. A list taller than the page breaks with its columns.
#let list-columns(parts, labels, g) = context {
  let n = g.columns
  let w = (g.width - (n - 1) * column-gap) / n
  let cols = balance(parts, n, w, labels, g)
  while cols.len() < n { cols.push(()) }
  grid(columns: (w,) * n, column-gutter: column-gap, ..cols.map(c => stack(c, labels, g)))
}

// The list above the steps (`above`: the card, and A4's list that goes
// first) in balanced columns over a hairline, A4's under its
// heading; else A4's rail beside the steps.
#let ingredients(r, ri, labels, g, card, above) = {
  let groups = r.ingredientGroups.enumerate().filter(((_, gr)) => gr.ingredients.len() > 0)
  if groups.len() == 0 { return none }
  let parts = groups.map(((gi, gr)) => ("g" + str(ri) + "-" + str(gi), gr))
  if above {
    if not card { section-heading(labels.ingredients) }
    block(width: 100%, inset: (top: if card { 3mm } else { 0pt }, bottom: 3mm), list-columns(parts, labels, g))
    line(length: 100%, stroke: hairline)
  } else {
    block(width: 100%, inset: (left: 5mm), stroke: (left: rail-rule), {
      section-heading(labels.ingredients)
      stack(parts, labels, g)
      column-end(ri)
    })
  }
}

// The facts row's entries: key, label, value and, for servings printed at
// another count than the recipe's, `from`: the long and short "scaled from
// n" notes, `{n}` in the labels replaced by the recipe's own servings.
#let facts(r, labels) = {
  let from = if r.servings.value != r.servings.original {
    let n = str(r.servings.original)
    (long: labels.scaledFrom.replace("{n}", n), short: labels.scaledFromShort.replace("{n}", n))
  }
  let list = (
    ("servings", r.servings.display),
    ("prep", r.times.prep),
    ("cook", r.times.cook),
    ("total", r.times.total),
  ).map(((k, v)) => (
    key: k, label: labels.at(k), value: if type(v) == dictionary { v.display } else { v },
    from: if k == "servings" { from },
  ))
  list = list.filter(f => f.value != none and f.value != "")
  (list, list.filter(f => f.key == "servings" or f.key == "total"))
}

// The card's meta: each fact's label and its value in bold ink, scaled
// servings with "(from n)".
#let meta-line(fs, size: 8pt) = {
  set text(size: size, fill: muted, ..lh(1.4))
  fs.map(f => {
    [#f.label #text(weight: 600, fill: ink, f.value)]
    if f.from != none { [~(#f.from.short)#metadata(f.from.short)<scaled-from>] }
  }).join(h(size))
}


// Lines whose last baseline is the block's bottom edge.
#let lines(..ls) = {
  let ls = ls.pos().filter(l => l != none)
  for (i, l) in ls.enumerate() {
    block(if i == ls.len() - 1 { text(bottom-edge: "baseline", l) } else { l })
  }
}

// The caption's widest line.
#let caption-max = 60mm

// The footer band: a hairline, then the source and credit on the left and
// "Open recipe", the host and the QR code on the right, all ending on the
// content's bottom edge. With the footer off only the hairline and the
// source; none when there is nothing to show. Its height is a constant. The
// QR code is bare: the 3 mm gaps above and beside it and the page margin are
// its quiet zone, four modules of 0.5 to 0.76 mm.
#let foot-band(data, ri, r, g) = {
  let on = data.options.footer
  let src = if r.source == none { none } else { r.source.display }
  if src == none and not on { return none }
  let labels = data.labels
  let b(v) = text(weight: 600, fill: text-ink, v)
  block(breakable: false, width: 100%, {
    context [#metadata((recipe: ri, page: here().page()))<footer>]
    line(length: 100%, stroke: hairline)
    v(3mm)
    set text(size: 8.5pt, fill: muted, ..lh(1.4))
    let credit = lines(
      if src != none {
        [#metadata(src)<source>]
        fill-line(v => [#labels.source #b(v)], src)
      },
      if on {
        let tail = if data.instance.attribution [~· #text(data.instance.product)]
        fill-line(v => [#labels.addedBy #b(v)#tail], r.author.displayName)
      },
    )
    if on and r.qr != none {
      let caption = {
        set text(size: 8pt, ..lh(1.35))
        set align(right)
        let line(make, v) = one-line(make, v, caption-max)
        lines(line(v => text(weight: 600, fill: text-ink, v), labels.openRecipe), line(v => v, host(r.qr.url)))
      }
      grid(columns: (1fr, auto, g.qr), column-gutter: 3mm, align: bottom,
        credit, caption, image(r.qr.path, width: g.qr, height: g.qr))
    } else { credit }
    // The band's bottom edge, for the tests.
    block(height: 0pt, context [#metadata((recipe: ri, page: here().page(), y: here().position().y.mm()))<footer-bottom>])
  })
}

// The footer band at the foot of the recipe's last page. With `kept`, the
// last step holds room below itself; the band backs up into it, from the
// `<body-end>` to the lower `<column-end>` on that page, so it never moves
// on alone. None of these positions depends on the band, so the layout
// converges; `here()` would not, as a context takes the position of the band
// after it.
#let foot(ri, band, kept, gap) = {
  column-end(ri, label: <body-end>)
  if band != none {
    if kept {
      context {
        let at = query(<body-end>).find(m => m.value == ri).location().position()
        let ys = query(<column-end>).filter(m => m.value == ri).map(m => m.location().position())
          .filter(p => p.page == at.page).map(p => p.y)
        if ys.len() > 0 { v(calc.max(..ys) - at.y) }
      }
    }
    v(gap)
    v(1fr)
    band
  }
  column-end(ri, label: <recipe-end>)
}

// The head: the tags, the title and, on A4, the description and the facts
// row. The card sets tags and meta on one baseline over a full-width title.
#let head(r, labels, card, outlined) = {
  let (all-facts, card-facts) = facts(r, labels)
  let tags = if r.tags.len() > 0 {
    let t = text(r.tags.join(" · "))
    [#metadata(t)<tags>]
    caps(t.text)
  }
  if card {
    let meta = if card-facts.len() > 0 { meta-line(card-facts) }
    // The title always has a row of its own, so it wraps alike everywhere.
    block(width: 100%, stroke: (bottom: structure), inset: (bottom: 2.5mm), {
      if tags != none or meta != none {
        block(below: 2pt, grid(columns: (1fr, auto), column-gutter: 6mm, align: (left + bottom, right + bottom),
          text(bottom-edge: "baseline", tags), text(bottom-edge: "baseline", meta)))
      }
      title(r, card, outlined)
    })
    return
  }
  if tags != none { block(below: 2.5pt, tags) }
  block(below: 3mm, title(r, card, outlined))
  if r.description != "" {
    let desc = text(r.description)
    [#metadata(desc)<description>]
    block(below: 4.5mm, width: 130mm, text(font: display, fill: muted, ..lh(1.45, f: display), desc))
  }
  if all-facts.len() > 0 {
    block(below: 6mm, stroke: (y: hairline), width: 100%, grid(
      columns: (1fr,) * all-facts.len(),
      stroke: (x, _) => if x > 0 { (left: hairline) },
      inset: (x, _) => (y: 3.5pt, left: if x > 0 { 8pt } else { 0pt }),
      ..all-facts.map(f => {
        caps(f.label, size: 7pt, weight: 500, tracking: 0.06em, h: 1.3)
        linebreak()
        text(font: display, size: 12pt, weight: 600, ..lh(1.2, f: display), f.value)
        if f.from != none {
          linebreak()
          text(size: 7.5pt, fill: muted, ..lh(1.3), f.from.long)
          [#metadata(f.from.long)<scaled-from>]
        }
      }),
    ))
  }
}

// The list above the steps: the card, and A4's list that goes first. On
// the card a method that fits a page is never split: what does not fit
// below the list starts on the next page whole, footer room and all, and a
// longer one flows on. On A4 the method flows on from below the list, so it
// starts on the recipe's first page. The marker before the method makes the
// decision independent of where the method lands.
#let stacked-body(r, ri, labels, g, card, reserve) = {
  let list = ingredients(r, ri, labels, g, card, true)
  list
  if r.steps.len() > 0 {
    let method = steps(r, ri, labels, g, card, reserve: reserve)
    if not card {
      block(above: if list == none { 0pt } else { 5mm }, width: measure-alone, method)
    } else {
      // Space after the list is weak, so a method that moves on starts at
      // the page's top.
      if list != none { v(3.5mm, weak: true) }
      column-end(ri, label: <method-start>)
      context {
        let at = query(<method-start>).find(m => m.value == ri).location().position()
        let h = measure(method, width: g.width).height
        if h <= g.margin.top + g.height - at.y or h > g.height { method } else { block(breakable: false, method) }
      }
    }
  }
}

// A4's steps left with the list beside them. The footer's room grows by
// `below`, what the list runs on below the steps (at most `list-first`, see
// `overhang`), so the band never lands beside the list. With the list
// running on below up to three steps, the room would carry the whole method
// off the list's page (the first step is sticky, and of three the second as
// well, as the second-to-last): then there is no room and the band follows
// the list.
// ponytail: the heights are unbroken, so on a recipe over several pages the
// room can be off by what the breaks leave blank.
// Returns the body and whether the last step kept the room it asked for.
#let side-body(r, ri, labels, g, reserve, below) = {
  let list = ingredients(r, ri, labels, g, false, false)
  let sw = if list == none { measure-alone } else { g.width - rail - rail-gap }
  let room = reserve
  if room != none and list != none {
    room = if below > 0pt and r.steps.len() <= 3 { none } else { room + calc.max(0pt, below) }
  }
  let method = steps(r, ri, labels, g, false, reserve: room)
  let body = if method == none { list } else if list == none { block(width: sw, method) } else {
    grid(columns: (sw, rail), column-gutter: rail-gap, method, list)
  }
  (body: body, kept: room != none)
}

// How far A4's rail would run on below the steps beside it, both measured
// unbroken; 0pt without a list or steps. Needs context.
#let overhang(r, ri, labels, g) = {
  let list = ingredients(r, ri, labels, g, false, false)
  if list == none or r.steps.len() == 0 { return 0pt }
  measure(list, width: rail).height - measure(steps(r, ri, labels, g, false), width: g.width - rail - rail-gap).height
}

// One recipe, from its photo to its footer.
#let recipe(data, ri, r, outlined: false) = {
  let card = is-card(data)
  let g = geometry(card)
  let labels = data.labels
  context [#metadata((recipe: ri, page: here().page()))<recipe-page>]
  if data.options.photo and r.photo != none {
    block(below: if card { 4mm } else { 6mm }, cover(r.photo, g.width, g.photo))
  }
  head(r, labels, card, outlined)
  let band = foot-band(data, ri, r, g)
  // The footer keeps with the last step: that step's unbreakable block holds
  // room for the band, so when the band does not fit after it, the step
  // moves to the next page with it, and no page holds the band alone.
  context {
    let g = g + ingredient-columns(r, g)
    let reserve = if band != none and r.steps.len() > 0 { g.gap + measure(band, width: g.width).height }
    let kept = reserve != none
    let below = if card { 0pt } else { overhang(r, ri, labels, g) }
    if card or below > list-first {
      stacked-body(r, ri, labels, g, card, reserve)
    } else {
      let side = side-body(r, ri, labels, g, reserve, below)
      side.body
      kept = side.kept
    }
    foot(ri, band, kept, g.gap)
  }
}
