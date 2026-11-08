# The mascot

`rezepte-mascot-original.jpg` is the mascot exactly as the generator returned it: a waving cookbook holding a wooden spoon, in vintage-cookbook gouache on its own cream ground. `rezepte-mascot.png` is the same picture with that ground flood-filled away from the four corners and trimmed (ImageMagick, `-fuzz 9%`), the master `mise run //frontend:brand:export` derives every lockup, icon and preview from. Both are in Git LFS.

`../icon/rezepte-book.svg`, the favicon, is the book alone: a flat redraw of the mascot, the face, arms, legs and spoon removed, traced with vtracer and mapped onto the palette in `frontend/scripts/brand-palette.ts`.

## Settings

| Setting       | Value                                                                                                     |
| ------------- | --------------------------------------------------------------------------------------------------------- |
| Tool          | `generate_image` of the [`mcp-image`](https://www.npmjs.com/package/mcp-image) MCP server, version 0.14.0 |
| Provider      | `gemini` (server default)                                                                                 |
| Model         | `gemini-3.1-flash-image`                                                                                  |
| `aspectRatio` | `1:1`                                                                                                     |
| `imageSize`   | `2K` (returns 2048 × 2048 px)                                                                             |

**purpose** for every step: Brand mascot and logo source for a recipe app, shown large on the login page and later simplified into an icon down to 16 px

The mascot took three steps, each an edit of the one before (`inputImagePath`, `maintainCharacterConsistency: true`).

## 1. The character

> Mascot illustration for a cooking app aimed at adults, in the style of a vintage cookbook illustration: gouache texture with a subtle dark ink outline, bold simplified shapes, warm muted colors with an amber-brown cover and a sage-green ribbon, no text, no letters, no frame, no border, no vignette. Subject: a hardcover cookbook character in a lively three-quarter pose, leaning slightly as it tips an imaginary hat, waving with one hand, a wooden spoon tucked under the other arm; the cover slightly open so a few cream pages fan out at the edge; face on the cover: two small, simple dark eyes, one brow slightly raised, a warm, knowing smile, no blush, no sparkle highlights. Characterful and friendly like a seasoned cook who is glad to see you, not cute, not childish. Full figure. Absolutely nothing else in the image. Centered, fills about 80% of the square. Flat, even, solid background color #f4ede2 (warm cream) filling the whole square edge to edge.

## 2. The spoon in hand, the palette

> Edit this illustration and keep the same cookbook character, same pose, same face and expression, same gouache style, same cream background #f4ede2, same framing. Two changes only: (1) the character's lower arm now clearly holds the wooden spoon, the hand visibly wrapped around the spoon's handle, the spoon resting diagonally against its body, naturally connected to the arm; (2) recolor the cover to a deep, rich amber-brown (between #b45309 and #7c3d0a) with the dark brown ink outline, keep the ribbon bookmark sage green (#7fb069) and the pages cream. No text.

## 3. A friendlier face (`rezepte-mascot-original.jpg`)

> Edit this illustration and keep everything the same: same cookbook character, same pose, same waving hand, same spoon, same colors, same gouache style, same background, same framing. Keep a little of its playful personality but make it clearly friendly: one eyebrow only very slightly higher than the other in a playful, good-humored way (not questioning), eyes open and warm, and a big, genuine, slightly lopsided grin that clearly reads as happy. Charming, fun and welcoming, not skeptical, not smug. No text.

## The book for the favicon

A flat redraw of step 3, then the book alone from it, both edits of the one before:

> Redraw this exact illustration as a clean flat vector illustration, tracing it as faithfully as possible: identical pose, identical proportions, identical tilt, identical waving hand, identical spoon grip, identical ribbon shape, and above all the identical face — same eyebrow shapes and positions (one very slightly higher), same eye shapes with their small highlights, same nose line, same lopsided happy grin. Only the rendering changes: replace the gouache texture with smooth flat color fills, one uniform bold dark-brown outline (#3b1d06) around every shape, at most one flat shadow tone per color. Colors: cover amber-brown #a8560b with shadow #8a4210, spine and corner details #8a4210, arms and legs #8a4210, wooden spoon #c47a30 with outline, ribbon sage green #86a86f with shadow #6f8f5c, pages cream #f1e6d2, face lines #3b1d06. Background pure flat #ffffff edge to edge. No text, no texture, no gradients.

> From this exact illustration, keep only the book itself: same cookbook, same colors, same outline weight, same flat vector style, same sage-green ribbon swooshing out of the top, same cream page block, same corner details. Remove the face completely (plain cover), remove the arms, hands, legs and the spoon. Stand the book upright and straight (no tilt), centered in the square, filling about 80% of its height. Clean flat shapes, no texture. Background pure flat #ffffff edge to edge. No text.

## The error page poses

Two poses for `frontend/src/routes/+error.svelte`, each an edit of `rezepte-mascot-original.jpg` with the same settings (`maintainCharacterConsistency: true`), generator model `gemini-nano-banana-2.1`. Neither is cut out: their loose pages and smoke are close to the ground's color, so the page shows each with its ground, as a card. `mise run //frontend:brand:export` moves that ground onto the app's background and writes them to `../lockups/rezepte-mascot-<pose>.webp` at 640 px.

**purpose**: Brand mascot pose for a recipe app's "page not found" / "something went wrong" error page

### `rezepte-mascot-not-found-original.jpg`

A first edit, then the spoon removed from it:

> Edit this illustration and keep the same cookbook character: same book, same amber-brown cover, same sage-green ribbon bookmark, same cream pages, same face style, same vintage-cookbook gouache style with dark ink outline, same flat cream background #f4ede2, same framing (full figure, centered, about 80% of the square). Change only the pose and expression: the book's cover is opened a little and the character peeks at its own fanned-out pages with one hand, while the other hand is raised palm up in a small shrug, the wooden spoon tucked under that arm. Expression: puzzled but good-humored, one eyebrow raised, a small crooked smile, as if saying "hm, that page isn't in me". Not sad, not cute, no blush, no sparkle highlights. No text, no question marks, no symbols.

> Edit this illustration and keep everything else exactly the same: same cookbook character, same pose, same hand lifting the page edge, same raised shrugging hand, same face and expression, same colors, same gouache style, same cream background #f4ede2, same framing. One change only: remove the wooden spoon completely; the arm that lifts the page simply lifts the page, nothing tucked under it. No text.

### `rezepte-mascot-error-original.jpg`

> Edit this illustration and keep the same cookbook character: same book, same amber-brown cover, same sage-green ribbon bookmark, same cream pages, same face style, same vintage-cookbook gouache style with dark ink outline, same flat cream background #f4ede2 edge to edge, same framing (full figure, centered, about 80% of the square). New scene: something just exploded on the stove. The book sits on the floor after being blown backwards, its top edge and the upper part of its cover blackened with soot, the page edges slightly singed and curled, a few wisps of grey smoke rising from its top; the sage ribbon is frizzled at the end. Beside it a small copper saucepan lies on its side, blackened inside, a thin curl of smoke coming out. The wooden spoon is still in one hand, its tip charred black. Face: soot smudges across the cover around the eyes, blinking, dazed, a sheepish lopsided grin with raised eyebrows, "oops, that went wrong", funny, not hurt, not scary, no fire, no flames, no blush, no sparkle highlights. Soft contact shadows. No text, no symbols, no motion lines.

## The empty shopping lists pose

### `rezepte-mascot-shopping-original.jpg`

For the shopping lists overview while a person has no list yet (`NoteEmptyState` on `/lists`). An edit of `rezepte-mascot-original.jpg` with the same settings (`maintainCharacterConsistency: true`), generator model `gemini-nano-banana-2.1`, shown with its ground as a card like the error poses.

**purpose**: Brand mascot pose for a recipe app's empty shopping-lists page

> Edit this illustration and keep the same cookbook character: same book, same amber-brown cover, same sage-green ribbon bookmark, same cream pages, same face style, same vintage-cookbook gouache style with dark ink outline, same flat cream background #f4ede2 edge to edge, same framing (full figure, centered, about 80% of the square). Change only the pose and props: the character carries a small empty woven wicker shopping basket over one arm, and with the other hand holds up a short blank slip of cream paper and a little pencil, as if about to write down what to buy. Expression: cheerful, eager, one eyebrow slightly raised, a warm lopsided grin, "let's see what we need". The basket is empty. No wooden spoon. No text, no letters or scribbles on the paper, no symbols, no blush, no sparkle highlights. Soft contact shadow.

## The empty recipe overview pose

### `rezepte-mascot-first-recipe-original.jpg`

For the recipe overview while the instance holds no recipe (`NoteEmptyState` on `/`). Two edits with the same settings (`maintainCharacterConsistency: true`), generator model `gemini-nano-banana-2.1`: the first of `rezepte-mascot-original.jpg`, the second of the first's result. A first try opened the book wide and moved the face onto the blank pages; the face belongs on the cover, so the cover only opens at a corner.

**purpose**: Brand mascot pose for a recipe app's empty recipe overview (no recipes yet)

> Edit this illustration and keep the same cookbook character exactly: same book, same amber-brown cover with the SAME FACE ON THE FRONT COVER (eyes, brows, lopsided grin stay on the brown cover), same sage-green ribbon bookmark, same cream pages, same vintage-cookbook gouache style with dark ink outline, same flat cream background #f4ede2 edge to edge, same framing (full figure, centered, about 80% of the square). Change only the pose and props: the character stands upright facing us and with one hand lifts the edge of its front cover a little away from the page block, like opening a door a crack, so a few blank cream pages are visible on the side; with the other hand it holds up a little pencil, inviting someone to write the first recipe. The wooden spoon is tucked under that arm. Expression: eager and welcoming, eyebrows raised, warm lopsided grin. The pages are blank. No text, no letters, no scribbles, no symbols, no blush, no sparkle highlights. Soft contact shadow.

> Edit this illustration and keep everything else exactly the same: same cookbook character, same face on the front cover, same expression, same raised right hand holding the little pencil, same lifted lower-right corner of the cover showing blank cream pages, same sage-green ribbon, same colors, same vintage gouache style with dark ink outline, same flat cream background #f4ede2 edge to edge, same framing. Two changes only: (1) remove the wooden spoon completely; (2) the other arm now reaches across in front of the body and its hand holds the lifted lower-right corner of its own cover, clearly peeling the cover open by itself so the blank pages beneath are visible. Only that one corner is open; the left side of the book stays closed. The pages are blank. No text, no letters, no scribbles, no symbols, no blush, no sparkle highlights.
