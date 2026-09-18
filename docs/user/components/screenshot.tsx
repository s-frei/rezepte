import { existsSync } from 'node:fs';
import path from 'node:path';
import { ImageZoom } from 'fumadocs-ui/components/image-zoom';

type Props = {
	/** Screenshot name; must match a test('<name>') in screenshots/. */
	name: string;
	/** Shown below the image and used as alt text. */
	caption: string;
	/** Show the Pixel 7 variants in a phone frame instead of the desktop ones. */
	mobile?: boolean;
};

const DESKTOP = { width: 1440, height: 900 };
const MOBILE = { width: 412, height: 915 };

function hasPng(file: string): boolean {
	return existsSync(path.join(process.cwd(), 'public', 'screenshots', file));
}

/**
 * Renders a generated screenshot pair (light for light mode, dark for dark
 * mode). Server component: at build time it checks public/screenshots for
 * `<name>-<device>-<scheme>.png` and renders a neutral placeholder frame
 * with the caption for every variant that is missing, so pages can reference
 * screenshots before `mise run //docs/user:screenshots` has produced them.
 */
export function Screenshot({ name, caption, mobile = false }: Props) {
	const size = mobile ? MOBILE : DESKTOP;
	const device = mobile ? 'mobile' : 'desktop';
	const frame = mobile
		? 'mx-auto w-[320px] overflow-hidden rounded-[28px] border-8 border-fd-border'
		: 'overflow-hidden rounded-xl border border-fd-border';
	return (
		<figure className="my-6">
			{(['light', 'dark'] as const).map((scheme) => {
				const file = `${name}-${device}-${scheme}.png`;
				const visibility = scheme === 'light' ? 'dark:hidden' : 'hidden dark:block';
				// The placeholder centres its text, so its dark variant has to restore
				// `display: flex` rather than `block`, which would beat the `flex` utility.
				const placeholderVisibility = scheme === 'light' ? 'flex dark:hidden' : 'hidden dark:flex';
				if (hasPng(file)) {
					return (
						<ImageZoom
							key={scheme}
							src={`/screenshots/${file}`}
							alt={caption}
							width={size.width}
							height={size.height}
							className={`${frame} ${visibility} m-0`}
						/>
					);
				}
				return (
					<div
						key={scheme}
						role="img"
						aria-label={`${caption} (screenshot not generated yet)`}
						style={{ aspectRatio: `${size.width} / ${size.height}` }}
						className={`${frame} ${placeholderVisibility} items-center justify-center border-dashed bg-fd-muted p-4 text-center text-sm text-fd-muted-foreground`}
					>
						<span>
							Screenshot <code>{name}</code> ({device}, {scheme}) is not generated yet — run{' '}
							<code>mise run //docs/user:screenshots</code>.
						</span>
					</div>
				);
			})}
			<figcaption className="mt-2 text-center text-sm text-fd-muted-foreground">{caption}</figcaption>
		</figure>
	);
}
