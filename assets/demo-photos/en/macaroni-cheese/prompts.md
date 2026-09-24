# Macaroni Cheese: demo photos

AI-generated originals for the sample recipe "Macaroni Cheese", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> Homemade macaroni cheese in a rectangular cream ceramic baking dish fresh from the oven: a bubbling golden-brown crust of toasted breadcrumbs and melted cheddar, one corner spooned out revealing creamy macaroni in thick cheese sauce, a serving spoon resting in the dish. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The dish is small in the frame: it fills about 50% of the frame width, fully inside the frame with both ends visible and generous empty table on all sides so it survives a square center crop. Clean dish rim. Plain unbranded tableware. No text, no hands, no people.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the macaroni cheese from the reference image, shot from a low side angle almost at dish height: a serving spoon lifting a generous scoop of creamy macaroni out of the baking dish, long glossy strands of melted cheddar stretching from the spoon back down into the dish, the crisp golden breadcrumb crust in sharp focus at the edge of the scoop. The rest of the dish, the linen napkin and the window light fall into a soft, strongly blurred background. The spoon rests on the dish rim, nobody holds it. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

The spoon still came out floating, and the dish handle carries a small embossed word.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for macaroni cheese: a plain stainless steel saucepan with a smooth, thick, pale golden cheese sauce, a balloon whisk resting in it, grated cheddar melting into the surface. Next to the saucepan a colander of drained cooked macaroni, a small bowl of grated cheddar, a small bowl of breadcrumbs and the empty rectangular cream ceramic baking dish waiting. Three-quarter overhead angle, the saucepan centered with calm space around it. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no hands, no people.
