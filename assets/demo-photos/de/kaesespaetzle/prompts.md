# Käsespätzle: demo photos

AI-generated originals for the sample recipe "Käsespätzle", kept exactly as the generator returned them. `1.jpg` is the cover.

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

Made in two steps. The first prompt below put a fork in the frame that lifted a portion with cheese strands and floated high above the pan, held by nothing. The kept photo is that result edited in place: the second prompt was sent with `inputImagePath` pointing at the first result and without `maintainCharacterConsistency`. Asking for a lifted portion invites floating cutlery; keep cheese strands for a close-up where the spoon can rest in the pan.

**purpose**

> Cover photo for a recipe in a home cookbook app, shown 4:3 on the detail page and center-cropped to a square on recipe cards

**prompt**

> Homemade Swabian Käsespätzle served in a black cast-iron pan: soft, irregular, golden egg noodles layered with melted mountain cheese, long stretchy cheese strands where a portion has been lifted out, topped with a generous heap of crisp, golden-brown fried onion rings and a sprinkle of fresh chives. A small cream bowl of green leaf salad beside the pan for colour. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The pan is small in the frame: it fills about 50% of the frame width, fully inside the frame with its handle, with generous empty table on all sides so it survives a square center crop. Plain unbranded pan and tableware. Each item appears once. No text, no labels, no hands, no people.

**edit prompt** (input: the result of the prompt above)

> Edit this photo. Remove the wooden fork completely, together with the portion of Spätzle it lifts and the long cheese strands hanging from it. Fill that area naturally: the cast-iron pan full of golden Spätzle with melted cheese, crisp fried onion rings and chives continues where the lifted portion was, with the pan's surface intact, and behind it the grey linen napkin and the window, as the rest of the photo shows. Keep everything else exactly as it is: the same pan and handle, the same Spätzle, onions and chives, the same bowl of salad, the same napkin, crumbs, table, light and framing. Do not add any cutlery or any new object. No text, no brand names.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the Käsespätzle from the reference image, shot from a low side angle almost at pan height: a plain metal serving spoon lies inside the cast-iron pan, resting on the Spätzle with its handle propped on the pan rim, and a scoop of Spätzle has just been turned over beside it, showing long, glossy strands of melted mountain cheese stretching between the soft golden noodles, crisp fried onion rings and chives on top in sharp focus. The rest of the pan, the salad bowl and the window light fall into a soft, strongly blurred background. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for Käsespätzle, three-quarter overhead angle: a wide plain stainless steel pot of simmering salted water in which freshly made, irregular pale yellow Spätzle float to the surface, a traditional wooden Spätzle board (Spätzlebrett) with a little sticky, glossy dough and a metal scraper resting across the pot's rim. Beside the pot a mixing bowl with the rest of the thick, bubbly Spätzle dough and a wooden spoon in it, and a small bowl of grated mountain cheese. Only these items, each appearing once. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no brand names, no hands, no people.
