# Kartoffelpuffer mit Apfelmus: demo photos

AI-generated originals for the sample recipe "Kartoffelpuffer mit Apfelmus", kept exactly as the generator returned them. `1.jpg` is the cover.

## Settings

Every photo in this folder was generated with these settings. To make an adjusted version, reuse them and change only the prompt.

| Setting       | Value                                                                                                     |
| ------------- | --------------------------------------------------------------------------------------------------------- |
| Tool          | `generate_image` of the [`mcp-image`](https://www.npmjs.com/package/mcp-image) MCP server, version 0.14.0 |
| Provider      | `gemini` (server default)                                                                                 |
| Model         | `gemini-3.1-flash-image`                                                                                  |
| `quality`     | `balanced`                                                                                                |
| `aspectRatio` | `4:3`                                                                                                     |
| `imageSize`   | `2K` (returns 2400 × 1792 px)                                                                             |
| Output        | JPEG                                                                                                      |
| Generated     | 2026-09-24                                                                                                |

Photos 2 and 3 also pass `inputImagePath` pointing at `1.jpg` and `maintainCharacterConsistency: true`, so they keep the cover's table, light and tableware.

## 1.jpg: cover

**purpose**

> Cover photo for a recipe in a home cookbook app, shown 4:3 on the detail page and center-cropped to a square on recipe cards

**prompt**

> Homemade German Kartoffelpuffer with Apfelmus on a cream plate: three thin, round potato pancakes with lacy, deep golden, crispy edges and visible grated potato strands, slightly overlapping, beside a generous dollop of chunky homemade apple sauce dusted with a little cinnamon, a few thin slices of fresh red apple for colour. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The plate is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. No cutlery on the plate or in the air. Plain unbranded tableware. Each item appears once. No text, no labels, no hands, no people.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the Kartoffelpuffer from the reference image, shot from a low side angle almost at plate height: one potato pancake broken in half, showing its soft, steaming grated-potato inside and the thin, lacy, shattering crisp golden edge in sharp focus, a spoonful of chunky apple sauce with a dusting of cinnamon resting on the broken pancake. The other pancakes, the apple slices and the window light fall into a soft, strongly blurred background. No cutlery in the shot. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

The broken inside came out with stretchy strands like melted cheese; ask for "soft, fluffy grated potato, no stretchy strands" to avoid that.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

Made in two steps. The first prompt below left the potato peel on the board as a closed ring, which peeling never produces. The kept photo is that result edited in place: the second prompt was sent with `inputImagePath` pointing at the first result and without `maintainCharacterConsistency`.

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for Kartoffelpuffer, three-quarter overhead angle: a plain black frying pan with bubbling hot oil and three flat potato pancakes frying in it, their edges turning golden and lacy, next to it a mixing bowl of raw grated potato and onion batter with a spoon in it, and a box grater with a few potato shreds on a board. Only these items, each appearing once, with a little natural mess: some potato shreds and a peel on the board. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no brand names, no hands, no people.

**edit prompt** (input: the result of the prompt above)

> Edit this photo. Keep the entire image exactly as it is: the same box grater, the same wooden board with potato shreds, the same frying pan with the three potato pancakes, the same bowl of batter with the spoon, the same table, window, plant, light and framing. Change only the potato peel on the board: replace the closed ring-shaped peel with a few loose, thin, irregular strips of potato peel as a vegetable peeler leaves them, each strip curled slightly, with open ends, lying flat and scattered on the board. Do not add, remove or duplicate anything else. No text, no brand names.
