# Sticky Toffee Pudding: demo photos

AI-generated originals for the sample recipe "Sticky Toffee Pudding", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> A homemade sticky toffee pudding served warm: a square of dark, moist date sponge on a cream dessert plate, drenched in glossy, amber toffee sauce that has already been poured over and runs down the sides and pools around it, a scoop of vanilla ice cream just starting to melt beside it. Behind the plate the rest of the pudding in a cream ceramic baking dish and a small cream jug of extra toffee sauce standing upright. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The plate is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. Plain unbranded tableware. No text, no labels, no hands, no people.

The plate came out smaller than asked, so the dessert fills only about a third of the square card crop.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Extreme close-up of the sticky toffee pudding from the reference image, shot from a low side angle almost at plate height: a dessert spoon has cut into the square of warm date sponge and rests on the plate, revealing the dark, moist, tender crumb studded with soft chopped dates, glossy amber toffee sauce soaking into the cut and dripping down the side into a shiny pool on the cream plate, the melting vanilla ice cream glistening beside it. The baking dish, the jug and the window light fall into a soft, strongly blurred background. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for sticky toffee pudding, three-quarter overhead angle: a plain stainless steel saucepan of smooth, glossy, amber toffee sauce made from butter, brown sugar and cream, a wooden spoon resting in it. Next to it the same cream ceramic baking dish with the freshly baked, risen, dark date sponge, half of its top already glossy with the toffee sauce that has been poured over. Around them a small bowl of chopped pitted dates, a small bowl of soft brown sugar, a block of butter on a saucer and a small jug of double cream. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no hands, no people.
