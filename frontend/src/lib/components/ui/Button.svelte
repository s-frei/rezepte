<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ResolvedPathname } from '$app/types';
	import { Button as BitsButton } from 'bits-ui';

	type Variant = 'primary' | 'secondary' | 'accent' | 'ghost' | 'destructive';
	type Size = 'sm' | 'md' | 'lg';

	let {
		variant = 'primary',
		size = 'md',
		type = 'button',
		disabled = false,
		busy = false,
		href,
		title,
		label,
		onclick,
		class: className = '',
		children
	}: {
		variant?: Variant;
		size?: Size;
		type?: 'button' | 'submit' | 'reset';
		disabled?: boolean;
		/**
		 * The request this button sent is still out: it stays at full
		 * strength (`aria-busy`, not disabled), and a sheen runs through it
		 * over a line that fills along its foot. Reduced motion shows neither;
		 * the caller's label says it then. The caller ignores a second press.
		 */
		busy?: boolean;
		/** Renders an `<a>` styled like the button instead of a `<button>`. Pass
		 * the result of `resolve()` from `$app/paths`, optionally with a `#anchor`. */
		href?: ResolvedPathname | `${ResolvedPathname}#${string}`;
		title?: string;
		/** Accessible name, when the visible text needs more context. */
		label?: string;
		onclick?: (event: MouseEvent) => void;
		class?: string;
		children: Snippet;
	} = $props();

	const variantClasses: Record<Variant, string> = {
		primary: 'bg-primary text-primary-foreground',
		secondary: 'border border-border bg-surface text-text',
		accent: 'bg-accent text-accent-foreground',
		ghost: 'bg-transparent text-text-muted hover:bg-surface',
		destructive: 'bg-destructive text-destructive-foreground'
	};

	// `sm` is 36px, for a button inside a field's box.
	// `md` is 40px everywhere.
	// `lg` is 48px, bumped to 56px on small screens for mobile primary CTAs.
	const sizeClasses: Record<Size, string> = {
		sm: 'h-9 px-4 text-body-sm',
		md: 'h-10 px-5 text-body-sm',
		lg: 'h-12 max-sm:h-14 px-6 text-body'
	};

	// The sheen and the line take the button's text color, which reads on
	// the primary fill in both themes.
	const busyClasses =
		'relative overflow-hidden motion-safe:after:absolute motion-safe:after:inset-0 motion-safe:after:bg-linear-[105deg] motion-safe:after:from-transparent motion-safe:after:from-30% motion-safe:after:via-current/40 motion-safe:after:to-transparent motion-safe:after:to-70% motion-safe:after:bg-size-[220%_100%] motion-safe:after:animate-sheen motion-safe:before:absolute motion-safe:before:inset-x-0 motion-safe:before:bottom-0 motion-safe:before:h-0.75 motion-safe:before:origin-left motion-safe:before:bg-current/50 motion-safe:before:animate-progress';

	const classes = $derived(
		`${busy ? busyClasses : ''} inline-flex items-center justify-center gap-2 rounded-pill font-semibold transition hover-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] ${variantClasses[variant]} ${sizeClasses[size]} ${className}`
	);
</script>

{#if href}
	<!-- The prop's type already demands resolve() output; the rule only loses
	     track of it once a `#anchor` may follow. -->
	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<a
		{href}
		{title}
		aria-label={label}
		aria-disabled={disabled}
		tabindex={disabled ? -1 : undefined}
		class="{classes} {disabled ? 'pointer-events-none opacity-50' : ''}"
	>
		{@render children()}
	</a>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
{:else}
	<BitsButton.Root
		{type}
		{disabled}
		aria-busy={busy || undefined}
		{title}
		aria-label={label}
		{onclick}
		class="{classes} disabled:pointer-events-none disabled:opacity-50"
	>
		{@render children()}
	</BitsButton.Root>
{/if}
