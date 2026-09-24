# Full English Breakfast: demo photos

AI-generated originals for the sample recipe "Full English Breakfast", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> A homemade full English breakfast on a large cream plate: two browned pork sausages, two rashers of crisp bacon, a fried egg with a glossy runny yolk, baked beans in tomato sauce, a halved grilled tomato, golden fried sliced mushrooms and two triangles of buttered toast, seasoned with black pepper. A mug of tea beside the plate. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The plate is centered and fills about 55% of the frame width, with generous empty table on all sides so it survives a square center crop. No text, no hands, no people.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

A first attempt ("the same plate, a little closer, being eaten") came out almost identical to the cover. A gallery photo needs a clearly different angle or distance from the cover.

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the full English breakfast from the reference image, shot from a low side angle almost at table height: a triangle of buttered toast dipping into a broken fried egg, the runny golden yolk flowing over the white, a few baked beans in tomato sauce and a sliced piece of browned sausage in sharp focus in the foreground. The rest of the cream plate, the speckled mug of tea and the window light fall into a soft, strongly blurred background. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 and center-cropped to a square

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for a full English breakfast: a large black cast-iron frying pan on a wooden board, browned pork sausages, crisp bacon rashers and golden sliced mushrooms pushed to one side, two eggs frying with set white and bright yolks and two halved tomatoes cut side down on the other side. Next to the pan a small saucepan of baked beans and two slices of bread. Three-quarter overhead angle, the frying pan centered and filling about 55% of the frame width with calm space around it so it survives a square center crop. Realistic home cooking photography, warm muted tones. No text, no hands, no people.
