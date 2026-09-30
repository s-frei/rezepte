# Scenes

Wide illustrations of the mascot for the README and the user docs, exactly as the generator returned them, in Git LFS. `kitchen.jpg` heads the README, `guests.jpg` the docs home; `laptop.jpg` and `phone.jpg` carry a chroma-green screen; `mise run //docs/user:hero-images` puts the current docs screenshots onto it, in perspective, and writes the finished pictures (see `docs/user/scripts/hero-images.ts`).

## Settings

| Setting                        | Value                                                                                                     |
| ------------------------------ | --------------------------------------------------------------------------------------------------------- |
| Tool                           | `generate_image` of the [`mcp-image`](https://www.npmjs.com/package/mcp-image) MCP server, version 0.14.0 |
| Provider                       | `gemini` (server default)                                                                                 |
| Model                          | `gemini-3.1-flash-image`                                                                                  |
| `aspectRatio`                  | `21:9`                                                                                                    |
| `imageSize`                    | `2K` (returns 3168 × 1344 px)                                                                             |
| `inputImagePath`               | `../mascot/rezepte-mascot-original.jpg`                                                                   |
| `maintainCharacterConsistency` | `true`                                                                                                    |

## kitchen.jpg

**purpose**: Wide hero banner at the top of a GitHub README and a documentation home page, with the app name set over the empty right third

> Use the attached character exactly (same waving cookbook mascot, same face, same colors, same ribbon, same wooden spoon, same vintage-cookbook gouache style with a subtle dark ink outline). New wide banner illustration: the mascot stands on a warm wooden kitchen counter in a cozy, sunlit home kitchen, waving at the viewer. Around it, arranged with calm spacing: a small enamel pot gently steaming, a bunch of fresh dill, a sprig of marjoram with pale pink flowers, two cinnamon sticks, a few loose handwritten recipe cards (illegible squiggles only), a ceramic bowl. Soft cream and warm amber tones, a pale sage-green tiled wall behind. The whole right third of the image is a calm, uncluttered cream wall area with nothing in it, reserved for a title. No text, no letters, no frame, no border. Painterly, warm, inviting, not cluttered.

## guests.jpg

**purpose**: Wide hero banner at the top of a GitHub README and a documentation home page, with the app name set over the empty right third

> Use the attached character exactly (same waving cookbook mascot, same face, same colors, same ribbon, same wooden spoon, same vintage-cookbook gouache style with a subtle dark ink outline). New wide banner illustration in the style of a vintage cookbook endpaper: the mascot on the left, sitting cheerfully on a small stack of old cookbooks on a wooden table, waving. Three tiny friendly guests keep it company: a little dill sprite with a yellow-green umbel as hair, a round marjoram sprite with pink flowers, and a small cinnamon-bark sprite with a star anise badge, all in the same gouache style. A steaming teapot and a few recipe cards (illegible squiggles only) on the table. Warm cream background with a faint pattern of hand-painted herbs. The right third of the image is calm, plain cream with nothing in it, reserved for a title. No text, no letters, no frame, no border. Charming, warm, grown-up rather than childish.

Then an edit of that picture (`inputImagePath` the first result), so the table runs through:

**purpose**: Wide hero banner at the top of a documentation home page, with the app name set over the calm right third

> Edit this illustration and keep everything the same (the same mascot on the same stack of cookbooks, the same three little guests, the same teapot and recipe cards, the same style and colors). One change only: the wooden table now runs across the entire width of the image all the way to the right edge, one continuous tabletop like a long kitchen table, and the faint hand-painted herb pattern on the cream wall continues softly across the whole background so the picture has no hard edge anywhere. The right third above the table stays calm and uncluttered: only the pale wall and the bare tabletop, nothing standing there, reserved for a title. No text, no letters, no frame, no border.

## laptop.jpg

**purpose**: Wide hero banner for a README and a docs home page; the green laptop screen will be replaced by a real app screenshot

> Use the attached character exactly (same waving cookbook mascot, same face, same colors, same ribbon, same wooden spoon, same vintage-cookbook gouache style with a subtle dark ink outline). New wide banner illustration: on a warm wooden kitchen table, an open modern laptop seen perfectly straight from the front (no perspective, the screen a flat rectangle facing the viewer), and the mascot standing to the left of it, presenting the laptop with one open hand as if proudly showing it. The laptop screen is filled edge to edge with pure flat chroma green #00FF00, completely uniform, no reflections, no content. A few herbs, a small steaming pot and recipe cards (illegible squiggles only) around. Cozy cream kitchen background with soft light. The far right fifth of the image is calm plain cream. No text, no letters, no frame, no border.

## phone.jpg

**purpose**: Wide hero banner for a README and a docs home page; the green phone screen will be replaced by a real app screenshot

> Use the attached character exactly (same waving cookbook mascot, same face, same colors, same ribbon, same wooden spoon, same vintage-cookbook gouache style with a subtle dark ink outline). New wide banner illustration: in a cozy kitchen, a smartphone stands upright on a little wooden stand on the counter, seen perfectly straight from the front (no perspective, the screen a flat upright rectangle facing the viewer), next to a steaming pot. The mascot stands beside it, pointing at the phone with the wooden spoon and smiling, as if cooking along from the recipe on it. The phone screen is filled edge to edge with pure flat chroma green #00FF00, completely uniform, no reflections, no content. Fresh herbs and a few ingredients on the counter, warm cream and sage tones. The right third of the image is calm plain cream wall, reserved for a title. No text, no letters, no frame, no border.
