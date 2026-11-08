<script lang="ts">
	import type { ShareLifetime } from '#lib/api/shares.js';
	import { m } from '#lib/paraglide/messages.js';
	import { lifetimeLabel } from '#lib/recipe/lifetimes.js';
	import { lifetimeCell } from '#lib/recipe/share-stub.js';

	// The lifetime bar: one equal cell per lifetime, the current one amber
	// like the ticket's stub. Before the link exists it is a radio choice;
	// once it exists every cell means "this long, from now", so the current
	// cell re-times the link too - a radio does nothing when re-checked,
	// which is why `retime` draws pressed buttons instead. The radios are
	// native ones, not a Bits UI RadioGroup: the bar sits inside the
	// audience choice's RadioGroup, whose arrow keys took nested Bits radios
	// for its own and dropped the audience rows from the tab order.
	let {
		options,
		value,
		mode,
		disabled = false,
		onpick
	}: {
		options: ShareLifetime[];
		value: ShareLifetime;
		mode: 'choose' | 'retime';
		/**
		 * `retime` only, since only a link that exists can be paused:
		 * `aria-disabled`, like `SegmentedControl`: focusable, every pick dropped.
		 */
		disabled?: boolean;
		/** `choose`: a different cell was picked; `retime`: any cell, the current one too. */
		onpick: (days: ShareLifetime) => void;
	} = $props();

	const uid = $props.id();
	const bar = 'grid auto-cols-fr grid-flow-col gap-1 rounded-md bg-background p-1';

	function cellClass(current: boolean): string {
		return `flex h-11 flex-col items-center justify-center gap-0.5 rounded-[calc(var(--radius-md)-3px)] transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
			current
				? 'bg-accent text-accent-foreground shadow-card'
				: disabled
					? 'text-text-muted'
					: 'text-text'
		}`;
	}
</script>

{#snippet face(days: ShareLifetime, current: boolean)}
	{@const cell = lifetimeCell(days)}
	<span class="font-display text-body-lg/none font-semibold">{cell.value}</span>
	{#if cell.unit}
		<span class="text-[11px]/none font-semibold {current ? '' : 'text-text-muted'}">
			{cell.unit}
		</span>
	{/if}
{/snippet}

{#if mode === 'choose'}
	<div role="radiogroup" aria-label={m.public_share_lifetime_label()} class={bar}>
		{#each options as days (days)}
			<!-- The radio lies over the whole cell, last so it takes the tap;
			     the cell draws its focus ring. -->
			<label
				class="relative {cellClass(
					days === value
				)} has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-primary"
			>
				{@render face(days, days === value)}
				<input
					type="radio"
					name={uid}
					checked={days === value}
					aria-label={lifetimeLabel(days)}
					onchange={() => onpick(days)}
					class="absolute inset-0 appearance-none opacity-0"
				/>
			</label>
		{/each}
	</div>
{:else}
	<div
		role="group"
		aria-label={m.public_share_lifetime_label()}
		aria-disabled={disabled || undefined}
		class={bar}
	>
		{#each options as days (days)}
			<button
				type="button"
				aria-pressed={days === value}
				aria-label={lifetimeLabel(days)}
				aria-disabled={disabled || undefined}
				onclick={() => {
					if (!disabled) onpick(days);
				}}
				class={cellClass(days === value)}
			>
				{@render face(days, days === value)}
			</button>
		{/each}
	</div>
{/if}
