# Demo avatars

AI-generated originals for the demo's people, kept exactly as the generator returned them. Each demo person is named after a pantry ingredient (`displayNames` in `service/internal/demo/seed.go`), and each picture draws on that ingredient: `seed/<username>.jpg` is the avatar the seed uploads, a creature made of the ingredient; `herbs/<username>.jpg` is the ingredient alone, generated alongside and kept as an alternative.

The backgrounds are mid-tones between the light and dark values of each person's palette color, so a picture reads on both themes: a picture cannot switch with the theme the way the color tokens do.

## Settings

| Setting       | Value                                                                                                     |
| ------------- | --------------------------------------------------------------------------------------------------------- |
| Tool          | `generate_image` of the [`mcp-image`](https://www.npmjs.com/package/mcp-image) MCP server, version 0.14.0 |
| Provider      | `gemini` (server default)                                                                                 |
| Model         | `gemini-3.1-flash-image`                                                                                  |
| `quality`     | server default                                                                                            |
| `aspectRatio` | `1:1`                                                                                                     |
| `imageSize`   | `2K` (returns 2048 × 2048 px)                                                                             |
| Output        | JPEG                                                                                                      |
| Generated     | 2026-09-30                                                                                                |

Every prompt passes **purpose**: Profile picture for a demo account, shown cropped to a circle from 20 px to 96 px on both light and dark app themes

## seed/demo.jpg: dill creature (amber, `#d6ad72`)

**prompt**: Avatar illustration for a household recipe app, in the style of a vintage cookbook or seed-packet illustration: gouache with a subtle dark ink outline, bold simplified shapes, few saturated colors, no text, no frame, no border. Subject: a small, friendly fictional creature made of dill, a gentle garden sprite whose head is crowned by a large yellow-green dill flower umbel like a shock of hair, with a soft green body, feathery dill fronds as little arms, and two calm dark dot eyes. Head-and-shoulders portrait, facing the viewer, charming but not cartoonish or kitschy. The creature is centered and fills about 70% of the square, so it still reads when shown at 20 pixels inside a circle. Flat, even, solid background color #d6ad72 (warm mid-tone amber) filling the whole square edge to edge.

## seed/mila.jpg: marjoram creature (clay, `#d49c80`)

**prompt**: Avatar illustration for a household recipe app, in the style of a vintage cookbook or seed-packet illustration: gouache with a subtle dark ink outline, bold simplified shapes, few saturated colors, no text, no frame, no border. Subject: a small, friendly fictional creature made of marjoram, a round little herb spirit whose body is a plump bunch of rounded soft green marjoram leaves, with pale pink knot-like marjoram flower clusters on top like a bonnet, and two calm dark dot eyes. Head-and-shoulders portrait, facing the viewer, charming but not cartoonish or kitschy. The creature is centered and fills about 70% of the square, so it still reads when shown at 20 pixels inside a circle. Flat, even, solid background color #d49c80 (warm mid-tone terracotta clay) filling the whole square edge to edge.

## seed/jonas.jpg: cinnamon creature (rose, `#d69ca3`)

**prompt**: Avatar illustration for a household recipe app, in the style of a vintage cookbook or seed-packet illustration: gouache with a subtle dark ink outline, bold simplified shapes, few saturated colors, no text, no frame, no border. Subject: a small, friendly fictional creature made of cinnamon, a sturdy little spice spirit whose body is a rolled cinnamon-bark scroll with the curled bark visible at the top like a hood, a star anise worn like a small badge, and two calm dark dot eyes. Head-and-shoulders portrait, facing the viewer, charming but not cartoonish or kitschy. The creature is centered and fills about 70% of the square, so it still reads when shown at 20 pixels inside a circle. Flat, even, solid background color #d69ca3 (warm mid-tone dusty rose) filling the whole square edge to edge.

## herbs/demo.jpg: dill (amber, `#d6ad72`)

**prompt**: Avatar illustration for a household recipe app, in the style of a vintage cookbook or seed-packet illustration: gouache with a subtle dark ink outline, bold simplified shapes, few saturated colors, no text, no face, no frame, no border. Subject: a single upright sprig of dill with one large, clearly shaped yellow-green flower umbel on a sturdy green stem, a few feathery leaves kept as bold simple strokes. The subject is centered and fills about 70% of the square, so it still reads when shown at 20 pixels inside a circle. Flat, even, solid background color #d6ad72 (warm mid-tone amber) filling the whole square edge to edge.

## herbs/mila.jpg: marjoram (clay, `#d49c80`)

**prompt**: Avatar illustration for a household recipe app, in the style of a vintage cookbook or seed-packet illustration: gouache with a subtle dark ink outline, bold simplified shapes, few saturated colors, no text, no face, no frame, no border. Subject: a small bunch of marjoram, three stems tied together with rounded soft green leaves and pale pink knot-like flower clusters at the tips. The subject is centered and fills about 70% of the square, so it still reads when shown at 20 pixels inside a circle. Flat, even, solid background color #d49c80 (warm mid-tone terracotta clay) filling the whole square edge to edge.

## herbs/jonas.jpg: cinnamon (rose, `#d69ca3`)

**prompt**: Avatar illustration for a household recipe app, in the style of a vintage cookbook or seed-packet illustration: gouache with a subtle dark ink outline, bold simplified shapes, few saturated colors, no text, no face, no frame, no border. Subject: two crossed cinnamon sticks, the rolled bark clearly visible at the cut ends, with one small star anise beside them. The subject is centered and fills about 70% of the square, so it still reads when shown at 20 pixels inside a circle. Flat, even, solid background color #d69ca3 (warm mid-tone dusty rose) filling the whole square edge to edge.
