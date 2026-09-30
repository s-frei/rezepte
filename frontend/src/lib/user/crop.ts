/** A square picked in the crop dialog, as fractions of the upright image:
 * `x` and `y` place its top-left corner against width and height, `size`
 * is its side against the shorter side - what `?crop=` sends. */
export type Crop = { x: number; y: number; size: number };
