# Oven Fish and Chips: demo photos

AI-generated originals for the sample recipe "Oven Fish and Chips", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> Homemade oven-baked fish and chips on a large cream plate: two golden, crisp beer-battered white fish fillets resting on a generous pile of thick-cut oven chips with browned edges and flakes of sea salt, a wedge of lemon beside the fish. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The plate is centered and fills about 55% of the frame width, with generous empty table on all sides so it survives a square center crop. No text, no hands, no people, no newspaper.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the fish and chips from the reference image, shot from a low side angle almost at plate height: one golden beer-battered fillet broken open with a fork, showing the steaming, bright white, flaky cod inside the thin crisp shell of batter, a squeeze of lemon glistening on it, a few thick-cut chips with crunchy browned edges and sea salt flakes in sharp focus in the foreground. The rest of the cream plate, the lemon wedge and the window light fall into a soft, strongly blurred background. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for oven fish and chips: a dark metal baking tray fresh out of the oven with golden roasted thick-cut chips spread out on one half and two golden battered white fish fillets on baking paper on the other half. Next to the tray a mixing bowl with smooth pale beer batter and a whisk resting in it, a small dish of flaky sea salt and a halved lemon. Three-quarter overhead angle, the baking tray centered and filling about 55% of the frame width with calm space around it. Realistic home cooking photography, warm muted tones. Plain unbranded tray and bowl. No text, no hands, no people.
