# Beef and Ale Stew: demo photos

AI-generated originals for the sample recipe "Beef and Ale Stew", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> A homemade beef and ale stew in a cream enamelled cast-iron casserole pot with the lid set aside: tender chunks of slow-braised beef in a glossy, dark, rich ale gravy with orange carrot pieces, celery and onion, a bay leaf on top, sprinkled with a little fresh chopped parsley. A wooden ladle rests in the pot. Beside it a chunk of crusty bread on a small board and a glass of brown ale. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The pot is centered and fills about 55% of the frame width, with generous empty table on all sides so it survives a square center crop. Plain unbranded pot and glass. No text, no hands, no people.

The pot came out larger than asked; the square card crop cuts its left handle.

## 2.jpg: served, top-down

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (served bowl, top-down) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> A single serving of the beef and ale stew from the reference image in a wide, shallow cream bowl, seen straight from above as a flat lay: tender beef chunks, orange carrots and celery in glossy dark ale gravy, fresh parsley scattered on top, a spoon resting in the bowl. Around the bowl on the same rustic wooden table: a torn piece of crusty bread, a folded linen napkin and the glass of brown ale seen from above. Top-down overhead view, the bowl centered, soft natural window light from the side, warm muted tones. Realistic home cooking photography. Plain unbranded tableware. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for beef and ale stew: the same cream enamelled cast-iron pot on a trivet, with flour-dusted beef cubes browning in hot oil, deep brown seared crust on the meat. Next to the pot a wooden board with chopped onion, carrot rounds and sliced celery, a small bowl of flour, two bay leaves and an open unlabelled brown bottle of ale beside a jug of beef stock. Three-quarter overhead angle, the pot centered with calm space around it. Realistic home cooking photography, warm muted tones. Plain unbranded bottle, pot and jug. No text, no labels, no hands, no people.

Most beef cubes came out still raw and floured rather than seared; to get a browned look, ask for "no raw red meat visible".
