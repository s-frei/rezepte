// The cookbook: a title page, the contents, then each recipe on a new page,
// with the folio "x / y" at the head from the contents on. Design in lib.typ.
#import "lib.typ": caps, cover, display, facts, geometry, hairline, is-card, lh, measure-alone, meta-line, muted, recipe, recipe-at, rule, setup, structure
#let data = json("data.json")
#let labels = data.labels
#let book-title = data.book.title
#set document(title: book-title)
#show: setup.with(data)

// Title page. With photos, up to three covers fill the page down to the
// title block at its foot (one whole, two side by side, or two over a
// wide one); without, the mascot stands as a seal over the title in the
// upper third. Then a short structure rule, the count and the date. The
// product name signs the foot over a hairline when the instance credits
// Rezepte (instance.attribution, as the recipe footer) and it is not the
// title already.
#let collage(photos, w, h, gap) = if photos.len() == 1 {
  cover(photos.at(0), w, h)
} else if photos.len() == 2 {
  grid(columns: 2, gutter: gap, ..photos.map(p => cover(p, (w - gap) / 2, h)))
} else {
  let row = (h - gap) / 2
  grid(columns: 2, gutter: gap,
    ..photos.slice(0, 2).map(p => cover(p, (w - gap) / 2, row)),
    grid.cell(colspan: 2, cover(photos.at(2), w, row)),
  )
}
#{
  let card = is-card(data)
  let w = geometry(card).width
  let photos = if data.options.photo {
    data.recipes.filter(r => r.photo != none).map(r => r.photo)
  } else { () }
  photos = photos.slice(0, calc.min(3, photos.len()))
  let name = data.instance.product
  let signed = data.instance.attribution and name != "" and lower(name) != lower(book-title)
  let title-block = {
    context [#metadata(here().position().y.mm())<book-title>]
    text(font: display, size: if card { 28pt } else { 44pt }, weight: 500, tracking: -0.015em,
      ..lh(1.05, f: display), book-title)
    block(above: if card { 5mm } else { 7mm }, below: if card { 4mm } else { 5mm }, line(length: 24mm, stroke: structure))
    if data.book != none {
      let line = (data.book.countDisplay, data.book.dateDisplay).filter(s => s != "").join(" · ")
      text(size: if card { 11pt } else { 13pt }, fill: muted, line)
    }
  }
  let signature = if signed {
    v(if card { 10mm } else { 16mm })
    line(length: 100%, stroke: hairline)
    block(above: 3mm, caps(name, size: 8pt))
    [#metadata(name)<book-signature>]
  }
  if photos.len() > 0 {
    block(height: 100%, grid(rows: (1fr, auto), row-gutter: if card { 8mm } else { 12mm },
      layout(size => [#metadata(size.height.mm())<book-collage>#collage(photos, w, size.height, 3pt)]),
      { title-block; signature }))
  } else {
    v(0.5fr)
    block(below: if card { 7mm } else { 10mm }, image("mascot.png", width: if card { 16mm } else { 24mm }))
    title-block
    v(1fr)
    signature
  }
  pagebreak()
}

// Contents: clickable entries with dotted leaders to the folio, each with
// the servings and total time under its title, set as the card's meta a
// little larger. The list stands a third down the free space, so a short
// contents never floats in the top of an empty page; a long one starts at
// the top and runs on. On A4 it keeps the steps' solo measure, left-aligned,
// so the leaders stay short enough to follow to the folio.
#let card = is-card(data)
#v(1fr)
#context [#metadata(here().position().y.mm())<toc-title>]
#block(below: if card { 8mm } else { 10mm },
  text(font: display, size: if card { 16pt } else { 20pt }, weight: 600, ..lh(1.2, f: display), labels.contents))
#show outline.entry: it => context {
  let at = it.element.location()
  let page = counter(page).at(at).first()
  [#metadata(page)<toc-entry>]
  let started = recipe-at(at.page())
  let meta = if started != none { facts(data.recipes.at(started.recipe), labels).at(1) } else { () }
  link(at, block(inset: (y: if card { 2.5mm } else { 3mm }), {
    layout(size => [#metadata(size.width.mm())<toc-width>])
    // Title, leaders and folio share the title's baseline.
    set text(bottom-edge: "baseline")
    grid(
      columns: (auto, 1fr, auto), column-gutter: 6pt, align: bottom,
      text(font: display, size: if card { 12pt } else { 13pt }, top-edge: lh(1.2, f: display).top-edge, it.element.body),
      box(width: 100%, line(length: 100%, stroke: (paint: rule, thickness: 0.8pt, dash: "dotted"))),
      text(weight: 600, size: 10pt, str(page)),
    )
    if meta.len() > 0 {
      [#metadata(meta.map(f => f.value))<toc-meta>]
      block(above: 4pt, meta-line(meta, size: 8.75pt))
    }
  }))
}
#block(width: if card { 100% } else { measure-alone }, outline(title: none))
#v(2fr)

#for (i, r) in data.recipes.enumerate() {
  pagebreak()
  recipe(data, i, r, outlined: true)
}
