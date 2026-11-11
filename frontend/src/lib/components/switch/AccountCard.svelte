<script lang="ts">
	import ChefHat from '@lucide/svelte/icons/chef-hat';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import KeyRound from '@lucide/svelte/icons/key-round';
	import X from '@lucide/svelte/icons/x';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { userColorClasses } from '#lib/user/color.js';
	import { nameSize } from './switch';

	let {
		person,
		current = false,
		expired = false,
		seen = '',
		editing = false,
		chosen = false,
		onpick,
		onremove
	}: {
		person: { id: string; displayName: string; color: string; avatarId: string | null };
		/** The active person: ringed, "Cooking now", never removable here. */
		current?: boolean;
		/** Its session ran out: dashed on the surface, "Sign in again". */
		expired?: boolean;
		/** When the person was last here (`seenLabel`), for everyone else. */
		seen?: string;
		editing?: boolean;
		/** Carries the view-transition name the splash takes over. */
		chosen?: boolean;
		onpick: () => void;
		/** Not for the active card, which has no ×. */
		onremove?: () => void;
	} = $props();

	const NAME_SIZES = {
		lg: 'text-body-lg min-[360px]:text-heading',
		md: 'text-body-lg',
		sm: 'text-body'
	};

	// The card stays flat in its own space (a tilt reached over its neighbors);
	// the avatar and, half as far, the name drift towards the pointer and a soft
	// light follows it. The values live on the element, so nothing re-renders.
	function follow(e: PointerEvent) {
		if (editing || e.pointerType !== 'mouse') return;
		const el = e.currentTarget as HTMLElement;
		const r = el.getBoundingClientRect();
		const x = (e.clientX - r.left) / r.width;
		const y = (e.clientY - r.top) / r.height;
		el.style.setProperty('--px', `${(x - 0.5) * 6}px`);
		el.style.setProperty('--py', `${(y - 0.5) * 6}px`);
		el.style.setProperty('--mx', `${x * 100}%`);
		el.style.setProperty('--my', `${y * 100}%`);
	}

	function unfollow(e: PointerEvent) {
		const el = e.currentTarget as HTMLElement;
		el.style.removeProperty('--px');
		el.style.removeProperty('--py');
	}
</script>

<button
	type="button"
	title={person.displayName}
	onclick={onpick}
	onpointermove={follow}
	onpointerleave={unfollow}
	class="card group relative grid min-h-[5.5rem] w-full grid-cols-[auto_minmax(0,1fr)_2.5rem] items-center gap-x-3 overflow-hidden rounded-index-card p-3 text-left shadow-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary min-[360px]:gap-x-4 {expired
		? 'expired border-2 border-dashed border-border bg-surface text-text'
		: userColorClasses(person.color)} {current ? 'current ring-[2.5px] ring-primary' : ''} {editing
		? 'editing'
		: 'hover:shadow-lift'}"
	style:view-transition-name={chosen ? 'chosen-card' : undefined}
>
	<span class="shine" aria-hidden="true"></span>
	<!-- A ring of the page's ground, so an initial never melts into its own card. -->
	<span
		class="avatar relative rounded-full bg-background p-0.5 min-[360px]:p-1 {expired
			? 'opacity-60 grayscale'
			: ''}"
	>
		<PersonMark {person} size="xl" />
	</span>
	<span class="label relative flex min-w-0 flex-col gap-1">
		<span
			data-name
			class="line-clamp-2 font-display leading-tight font-medium wrap-anywhere {NAME_SIZES[
				nameSize(person.displayName)
			]} {expired ? 'text-text-muted' : ''}">{person.displayName}</span
		>
		{#if current}
			<span class="flex items-center gap-1.5 text-caption font-semibold whitespace-nowrap">
				<ChefHat class="size-3.5" aria-hidden="true" />
				{editing ? m.switch_you() : m.switch_current()}
			</span>
		{:else if expired}
			<span
				class="flex items-center gap-1.5 text-caption font-semibold whitespace-nowrap text-accent-foreground"
			>
				<KeyRound class="key size-3.5" aria-hidden="true" />
				{m.switch_expired()}
			</span>
		{:else}
			<span class="truncate text-caption opacity-75">{seen}</span>
		{/if}
	</span>
	<span class="relative flex size-10 items-center justify-center">
		{#if !current && !editing}
			<ChevronRight
				class="size-5 opacity-50 transition group-hover:translate-x-1 group-hover:opacity-90"
				aria-hidden="true"
			/>
		{/if}
	</span>
</button>
<!-- Beside the card, not in it: a button cannot hold another. It covers the
     card's action column, which stays empty while editing. -->
{#if editing && !current}
	<button
		type="button"
		aria-label={m.switch_remove_label({ name: person.displayName })}
		onclick={onremove}
		class="remove absolute top-1/2 right-3 flex size-10 -translate-y-1/2 items-center justify-center rounded-full bg-destructive text-destructive-foreground shadow-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		<X class="size-4.5" strokeWidth={2.5} aria-hidden="true" />
	</button>
{/if}

<style>
	/* Ruled like an index card. */
	.card {
		background-image: repeating-linear-gradient(
			to bottom,
			transparent 0 23px,
			color-mix(in oklab, currentColor 9%, transparent) 23px 24px
		);
		background-position: 0 14px;
	}
	.card.expired {
		background-image: none;
	}
	.shine {
		position: absolute;
		inset: 0;
		opacity: 0;
		background: radial-gradient(
			circle at var(--mx, 50%) var(--my, 50%),
			color-mix(in oklab, var(--color-background) 45%, transparent),
			transparent 55%
		);
		pointer-events: none;
	}
	@media (prefers-reduced-motion: no-preference) {
		.card {
			transition:
				translate 0.2s ease-out,
				scale 0.2s ease-out,
				box-shadow 0.2s ease-out;
		}
		.avatar,
		.label {
			transition: translate 0.3s ease-out;
		}
		.shine {
			transition: opacity 0.3s;
		}
		@media (hover: hover) {
			.card:not(.editing):hover {
				translate: 0 -2px;
			}
			.card:not(.editing):hover .avatar {
				translate: var(--px, 0) var(--py, 0);
			}
			.card:not(.editing):hover .label {
				translate: calc(var(--px, 0px) / 2) calc(var(--py, 0px) / 2);
			}
			.card:not(.editing):hover .shine {
				opacity: 1;
			}
			.card.expired:hover :global(.key) {
				animation: jiggle 0.5s ease-in-out;
			}
		}
		.card:not(.editing):active {
			scale: 0.985;
		}
		.card.editing:not(.current) {
			animation: wobble 0.45s ease-in-out;
		}
		.remove {
			/* `pop` is app.css's. */
			animation: pop 0.35s cubic-bezier(0.3, 1.6, 0.5, 1) both;
		}
	}
	@keyframes jiggle {
		25% {
			rotate: -14deg;
		}
		75% {
			rotate: 10deg;
		}
	}
	@keyframes wobble {
		30% {
			rotate: -0.8deg;
		}
		70% {
			rotate: 0.6deg;
		}
	}
</style>
