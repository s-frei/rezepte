# Rinderrouladen: demo photos

AI-generated originals for the sample recipe "Rinderrouladen", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> A homemade German Sunday dinner of Rinderrouladen on a cream plate: two braised beef roulades tied with kitchen string, glossy under a rich dark brown gravy, one cut open showing the swirl of beef around bacon, mustard, onion and a gherkin strip, served with a fluffy potato dumpling (Kartoffelkloß) and a portion of glossy braised red cabbage for colour. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The plate is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. Plain unbranded tableware. No text, no labels, no hands, no people.

The plate came out larger than asked; the square card crop trims its rim on both sides.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the Rinderrouladen from the reference image, shot from a low side angle almost at plate height: one roulade sliced into thick rounds, the cut faces in sharp focus showing the tight spiral of tender braised beef around smoky bacon, a streak of mustard, soft onion and a bright green gherkin centre, glossy dark gravy running over them and pooling on the plate, a little red cabbage at the edge. The dumpling, the rest of the plate and the window light fall into a soft, strongly blurred background. No cutlery in the shot. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

Made in two steps. The first prompt below produced a good scene, but with the gherkin strips lying lengthwise on top of the slices, which cannot be rolled. A fresh generation with the fix duplicated the props and showed a real brand on the pot; a stripped-down one looked staged. The kept photo is the first result edited in place: the second prompt was sent with `inputImagePath` pointing at that first result and without `maintainCharacterConsistency`. For a detail fix, edit the photo rather than generate a new one.

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for Rinderrouladen, seen from above at a steep overhead angle: on a wooden board, two thin raw beef slices laid flat and spread with golden mustard, topped with rashers of bacon, strips of onion and a long gherkin strip, a third one already rolled up and tied with kitchen string. Beside the board a small jar-free bowl of mustard with a knife, a small bowl of gherkins, a ball of kitchen string, and a plain dark cast-iron braising pot waiting. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no hands, no people.

**edit prompt** (input: the result of the prompt above)

> Edit this photo. Keep the entire image exactly as it is: the same wooden board, the same two raw beef slices with mustard, bacon and onion, the same rolled and tied roulade, the same bowl of mustard with the knife, the same bowl of gherkins, the same ball of string, the same black pot and lid, the same table, light and framing. Change only the two gherkin strips lying on the two flat beef slices: move each one so it lies crosswise along the short left end of its beef slice, parallel to that short edge, pressed flat into the mustard and bacon, exactly where the rolling starts. Do not add, remove or duplicate anything else. No text, no brand names.
