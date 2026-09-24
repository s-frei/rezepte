# Toad in the Hole: demo photos

AI-generated originals for the sample recipe "Toad in the Hole", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> Homemade toad in the hole straight from the oven in a dark metal roasting tin: eight browned pork sausages nestled in a dramatically risen, puffed, crisp golden Yorkshire pudding batter with deep brown edges climbing up the sides of the tin. Beside the tin a small cream jug of glossy brown onion gravy. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The tin is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. Plain unbranded tin and jug. No text, no hands, no people.

## 2.jpg: served with gravy

Reference: `1.jpg`, `maintainCharacterConsistency: true`

A first attempt asked for gravy "being poured from the jug"; with hands excluded, the jug stood upright on the plate and poured by itself. Describe gravy as already poured, with the jug standing beside the plate.

**purpose**

> Second gallery photo (served portion with gravy, low angle) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> A single serving of the toad in the hole from the reference image on a cream plate, seen from a low side angle close to plate height: a square slice cut from the tin showing two browned sausages set in the airy, crisp, golden Yorkshire pudding with its soft, custardy inside visible at the cut edge. Glossy brown onion gravy with soft sliced onions has already been poured over one side and pools on the plate, a few buttered green peas beside it. The cream gravy jug stands upright on the table next to the plate, not pouring. The roasting tin blurred in the background. Same rustic wooden table and warm window light as the reference. Shallow depth of field, realistic home cooking photography, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for toad in the hole: the same dark metal roasting tin with eight lightly browned pork sausages sizzling in shimmering hot oil, next to it a large jug of smooth, pale yellow Yorkshire pudding batter with a whisk resting in it. Beside them a small frying pan of soft, golden, slowly fried sliced onions for the gravy, two eggshells, a small bowl of flour and a jug of beef stock. Three-quarter overhead angle, the roasting tin centered with calm space around it. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no hands, no people.
