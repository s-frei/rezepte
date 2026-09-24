# Sausage Rolls: demo photos

AI-generated originals for the sample recipe "Sausage Rolls", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> Six homemade sausage rolls fresh from the oven on a cream serving plate: puffed, flaky, deep golden egg-glazed puff pastry with a few diagonal slashes on top, one roll cut in half showing the juicy, sage-flecked sausage meat filling and the many thin layers of pastry. A small ramekin of brown sauce beside the plate. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The plate is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. Plain unbranded tableware. No text, no labels, no hands, no people.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Extreme close-up of one of the sausage rolls from the reference image, cut in half and standing on its cut end on the cream plate, shot straight at the cut face from a low side angle almost at plate height: the juicy, sage-flecked sausage meat filling steaming, surrounded by dozens of thin, crisp, shattering golden layers of puff pastry, a few pastry flakes scattered on the plate. The other rolls, the ramekin of brown sauce and the window light fall into a soft, strongly blurred background. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for sausage rolls, seen from above at a steep overhead angle: a sheet of unbaked pale puff pastry on a floured wooden board, already rolled around a long log of pink sage-flecked sausage meat and sealed, cut with a knife into six unbaked rolls with diagonal slashes, a small bowl of beaten egg with a pastry brush resting in it, a few rolls already glossy with egg wash. Beside the board a mixing bowl with the leftover sausage meat mixture with chopped onion and dried sage, a baking tray lined with baking paper waiting. Realistic home cooking photography, warm muted tones. Plain unbranded utensils. No text, no labels, no hands, no people.
