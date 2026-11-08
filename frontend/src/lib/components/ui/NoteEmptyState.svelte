<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		art,
		title,
		steps,
		children
	}: {
		/** A mascot pose from `$brand/lockups`, 640px square on the app's ground. */
		art: string;
		title: string;
		/** Each step as its line and the hint under it. */
		steps: [string, string][];
		/** The actions at the note's end. */
		children: Snippet;
	} = $props();
</script>

<!-- An empty page that explains itself: a mascot pose on a small stack of
     pages beside a taped note, whose rows are the steps that fill the page.
     `data-empty`: the recipe overview's glide names it, so it crosses over
     with the cards it replaces. -->
<div
	data-empty
	class="mx-auto grid max-w-md grid-cols-[minmax(0,1fr)] pt-11 md:max-w-none md:grid-cols-[15rem_24rem] md:items-center md:justify-center md:gap-12 md:pt-7 lg:grid-cols-[280px_24rem] lg:gap-16"
>
	<div class="art relative z-10">
		<span class="page page-back" aria-hidden="true"></span>
		<span class="page page-front" aria-hidden="true"></span>
		<div
			class="pic relative rounded-3xl border border-border bg-surface p-1.5 shadow-lift md:p-2.5"
		>
			<img
				src={art}
				alt=""
				width="320"
				height="320"
				class="block aspect-square w-full rounded-[10px] md:rounded-xl"
			/>
		</div>
	</div>

	<div
		class="note @container relative w-full rounded-t-md rounded-b-xl bg-surface px-5 pt-6.5 pb-4.5 shadow-lift md:px-6 md:pt-7.5 md:pb-5.5"
	>
		<span class="tape" aria-hidden="true"></span>
		<span class="art-room" aria-hidden="true"></span>
		<h2
			class="pb-3 font-display text-[1.375rem] leading-tight font-medium text-balance [hyphenate-limit-chars:6_3_3] min-[360px]:text-2xl md:text-[1.625rem] @max-[12rem]:hyphens-auto"
		>
			{title}
		</h2>
		<ol>
			{#each steps as [step, hint], i (i)}
				<li
					class="step flex items-start gap-3 border-t border-dashed border-border py-2 md:py-2.5"
					style:--i={i}
				>
					<span
						aria-hidden="true"
						class="mt-0.5 size-4.5 shrink-0 rounded-full border-2 border-handle"
					></span>
					<span class="min-w-0 text-body text-text">
						{step}
						<span class="block text-body-sm text-text-muted">{hint}</span>
					</span>
				</li>
			{/each}
		</ol>
		<!-- Actions at a card's end, as in a dialog: content-sized at the end
		     from md; below it they share the line and wrap when it is short. -->
		<div class="cta mt-4.5 flex flex-wrap gap-2.5 *:max-md:flex-1 md:justify-end">
			{@render children()}
		</div>
	</div>
</div>

<style>
	/* The painting lies on a small stack of pages, as on the error page. */
	.page {
		position: absolute;
		inset: 0;
		border-radius: var(--radius-3xl);
		border: 1px solid var(--color-border);
		background: var(--color-surface);
		box-shadow: var(--shadow-lift);
	}
	.page-back {
		transform: rotate(7deg) translate(5px, 3px);
	}
	.page-front {
		transform: rotate(-4deg) translate(-3px, 2px);
	}
	.pic {
		transform: rotate(4deg);
	}
	/* Below md the painting and the note share one grid cell, and the
	   painting sits in the note's top end corner, up to 128px, poking well over its
	   edge (80px went under on a phone). Placed
	   in the note's own cell, it lines up with the note whatever the page
	   around it does. Its size is in px, so larger text does not grow it. The
	   note floats an empty box under it (`art-room`): the title flows beside
	   the painting, and a word that no longer fits there, with larger text or
	   a longer language, moves below it instead of running underneath. */
	.art-room {
		display: none;
	}
	@media (max-width: 767.98px) {
		.art,
		.note {
			grid-area: 1 / 1;
		}
		.art {
			justify-self: end;
			align-self: start;
			/* 41% of the note, so a 320px phone keeps room for the title's
			   longest word beside it; 128px from a 360px phone up. */
			width: min(128px, 41%);
			margin: -40px -6px 0 0;
		}
		.art-room {
			display: block;
			float: right;
			/* The painting reaches 122px into the note from its end edge and
			   about 97px down from its top; the note's padding (20px and 26px)
			   is already off both. */
			width: min(104px, 38%);
			height: 74px;
		}
		/* The steps start below the painting, never in a sliver beside it. */
		ol {
			clear: both;
		}
	}
	.note {
		transform: rotate(-0.6deg);
	}
	/* A strip of tape holds the note; on a phone it keeps left of the mascot. */
	.tape {
		position: absolute;
		top: -10px;
		left: 30%;
		width: 84px;
		height: 22px;
		translate: -50% 0;
		rotate: 2deg;
		border-radius: 2px;
		border: 1px solid var(--color-border);
		background: var(--color-accent);
		opacity: 0.95;
	}

	@media (min-width: 768px) {
		.page-back {
			transform: rotate(6deg) translate(8px, 5px);
		}
		.page-front {
			transform: rotate(-3deg) translate(-5px, 3px);
		}
		.pic {
			transform: rotate(-1.5deg);
		}
		.note {
			transform: rotate(0.8deg);
		}
		.tape {
			left: 50%;
		}
	}

	/* One moment on arrival: the painting drops in and its pages fan out,
	   the note is pressed on, its rows are written one after another, and
	   the button rises last. Nothing moves under reduced motion. */
	@media (prefers-reduced-motion: no-preference) {
		.art {
			animation: drop 900ms var(--ease-spring) both;
		}
		.page {
			animation: fan 900ms 200ms var(--ease-spring) backwards;
		}
		.note {
			animation: press 700ms 250ms var(--ease-spring) both;
		}
		.tape {
			animation: stick 420ms 700ms var(--ease-glide) both;
		}
		.step {
			animation: write 520ms calc(750ms + var(--i) * 160ms) var(--ease-glide) both;
		}
		.step > span:first-child {
			animation: pop 360ms calc(750ms + var(--i) * 160ms) var(--ease-spring) both;
		}
		.cta {
			animation: rise 560ms 1300ms var(--ease-spring) both;
		}
	}

	@keyframes drop {
		from {
			opacity: 0;
			translate: 0 -40px;
			scale: 0.96;
		}
	}
	@keyframes fan {
		from {
			transform: none;
		}
	}
	@keyframes press {
		from {
			opacity: 0;
			translate: 0 18px;
			rotate: 2.5deg;
		}
	}
	@keyframes stick {
		from {
			opacity: 0;
			scale: 1.25;
		}
	}
	@keyframes write {
		from {
			clip-path: inset(0 100% 0 0);
		}
		to {
			clip-path: inset(0 0 0 0);
		}
	}
	@keyframes pop {
		from {
			scale: 0;
		}
	}
	@keyframes rise {
		from {
			opacity: 0;
			translate: 0 14px;
		}
	}
</style>
