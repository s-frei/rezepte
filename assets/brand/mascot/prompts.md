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
