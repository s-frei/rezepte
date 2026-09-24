# Lemon Drizzle Cake: demo photos

AI-generated originals for the sample recipe "Lemon Drizzle Cake", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> A homemade lemon drizzle loaf cake on a cream plate: a golden loaf with a crackly, sparkling white crust of set lemon sugar on top, a few curls of fresh lemon zest, one thick slice cut and leaning against the loaf showing the soft, pale yellow, syrup-soaked crumb. Beside the plate a halved lemon and a whole lemon with a leaf. Style: realistic home baking photography, rustic wooden table, soft natural side light from a window, warm, light and fresh tones, three-quarter overhead angle, shallow depth of field. The plate is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. Plain unbranded tableware. No text, no labels, no hands, no people.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of a single thick slice of the lemon drizzle cake from the reference image lying flat on a small cream plate, shot from a low side angle almost at plate height: the soft, moist, pale yellow crumb glistening with soaked-in lemon syrup, the crackly white lemon sugar crust along the top edge catching the light, a small dessert fork resting on the plate beside the slice with a bite-sized piece already cut off, a few crumbs. The loaf, the lemons and the window light fall into a soft, strongly blurred background. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home baking, light and fresh warm tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for lemon drizzle cake, seen from above at a steep overhead angle: the freshly baked golden loaf still in its plain metal loaf tin lined with baking paper, its top pricked all over with a skewer, glossy wet lemon syrup already poured over it and soaking into the holes, a few sugar crystals not yet set. Beside the tin a small jug with the rest of the lemon juice and sugar syrup, a wooden skewer, squeezed lemon halves, a lemon zester with a little pile of zest and a small bowl of caster sugar. Realistic home baking photography, light and fresh warm tones. Plain unbranded tin and utensils. No text, no labels, no hands, no people.
