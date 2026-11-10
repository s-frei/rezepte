<script module lang="ts">
	/**
	 * Grows the element named `chosen-card` (a picked card, the login's sheet
	 * or card) into the splash: a view transition whose `show` renders the
	 * splash, which takes the name over (`named`) while the old one lets it
	 * go. Without view transitions or under reduced motion `show` simply runs.
	 * The root rules below hang off `html[data-switching]`, set only for this
	 * transition: a page's styles stay loaded after it is left, and the app's
	 * own page transitions keep their cross-fade. `corners` is the growing
	 * element's border radius to start from (`--grow-from`); left out, an
	 * index card's.
	 */
	export async function grow(show: () => void, corners?: string): Promise<void> {
		if (!document.startViewTransition || matchMedia('(prefers-reduced-motion: reduce)').matches) {
			return show();
		}
		const root = document.documentElement;
		root.dataset.switching = '';
		if (corners) root.style.setProperty('--grow-from', corners);
		const transition = document.startViewTransition(show);
		// A skipped transition still runs `show`; only the animation is lost.
		transition.ready.catch(() => {});
		await transition.finished
			.catch(() => {})
			.finally(() => {
				delete root.dataset.switching;
				root.style.removeProperty('--grow-from');
			});
	}
</script>

<script lang="ts">
	import ChefHat from '@lucide/svelte/icons/chef-hat';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { userColorClasses } from '#lib/user/color.js';
	import { firstName } from './switch';

	let {
		person,
		named = false,
		greeted = false
	}: {
		person: { id: string; displayName: string; color: string; avatarId: string | null };
		/** Takes over the picked card's view-transition name while it grows. */
		named?: boolean;
		/** Greeted: the contents fade, the ground stays for the app's arrival. */
		greeted?: boolean;
	} = $props();

	const name = $derived(firstName(person.displayName));
</script>

<!-- The picked card, grown into the whole screen (the view transition shares
     its name), greeting the person while the app is set up for them. -->
<div
	role="status"
	class="fixed inset-0 z-60 flex flex-col items-center justify-center gap-5 px-6 text-center {userColorClasses(
		person.color
	)}"
	style:view-transition-name={named ? 'chosen-card' : undefined}
	class:greeted
>
	<span class="mark rounded-full bg-background p-2">
		<PersonMark {person} size="profile" />
	</span>
	<p
		class="hello max-w-full font-display font-medium text-balance hyphens-auto {name.length > 14
			? 'text-heading-lg md:text-display-sm'
			: 'text-display-lg'}"
	>
		{m.switch_hello({ name })}
	</p>
	<p class="sub flex items-center gap-2 text-body opacity-80">
		<ChefHat class="size-4" aria-hidden="true" />
		{m.switch_hello_sub()}
	</p>
</div>

<style>
	@media (prefers-reduced-motion: no-preference) {
		.mark {
			/* `pop` is app.css's. */
			animation: pop 0.6s 0.25s cubic-bezier(0.3, 1.5, 0.5, 1) both;
		}
		.hello {
			animation: rise 0.5s 0.35s cubic-bezier(0.2, 0.8, 0.3, 1) both;
		}
		.sub {
			animation: rise 0.5s 0.5s cubic-bezier(0.2, 0.8, 0.3, 1) both;
		}
		/* Replaces the way in, which holds its end with `both`. */
		.greeted > * {
			animation: away 140ms ease-out both;
		}
	}
	@keyframes away {
		to {
			opacity: 0;
			scale: 0.96;
		}
	}
	@keyframes rise {
		from {
			opacity: 0;
			translate: 0 14px;
		}
	}

	/* The grow (`grow` above): the named element grows into the splash. The
	   root does not cross-fade (only its live new image shows, where the
	   switch's other cards fall away); the new image is opaque from the start
	   while the old one fades, both filling the group as it grows, and the
	   pair's corners round off into the screen's. */
	:global(html[data-switching]::view-transition-old(root)),
	:global(html[data-switching]::view-transition-new(root)) {
		animation: none;
	}
	:global(html[data-switching]::view-transition-old(root)) {
		opacity: 0;
	}
	:global(::view-transition-old(chosen-card)),
	:global(::view-transition-new(chosen-card)) {
		height: 100%;
		object-fit: cover;
		object-position: 50% 50%;
	}
	:global(::view-transition-new(chosen-card)) {
		animation: none;
	}
	:global(::view-transition-old(chosen-card)) {
		animation-duration: 240ms;
		animation-timing-function: ease-out;
	}
	:global(::view-transition-group(chosen-card)) {
		animation-duration: 0.55s;
		animation-timing-function: cubic-bezier(0.2, 0.9, 0.25, 1);
	}
	/* Keyframes declared `-global-` are referenced by their bare name. */
	:global(::view-transition-image-pair(chosen-card)) {
		overflow: clip;
		animation: switch-corners 0.55s cubic-bezier(0.2, 0.9, 0.25, 1) both;
	}
	@keyframes -global-switch-corners {
		from {
			border-radius: var(--grow-from, var(--radius-index-card));
		}
		to {
			border-radius: 0;
		}
	}
</style>
