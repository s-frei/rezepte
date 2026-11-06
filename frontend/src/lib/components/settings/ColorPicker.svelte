<script lang="ts">
	import { RadioGroup } from 'bits-ui';
	import Check from '@lucide/svelte/icons/check';
	import type { PersonEntry } from '#lib/api/users.js';
	import FaceStack from '#lib/components/ui/FaceStack.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { USER_COLORS, userColorClasses, type UserColor } from '#lib/user/color.js';
	import { userColorLabel } from '#lib/user/color-label.js';
	import { colorHolders, holdersFullText, holdersText } from '#lib/user/holders.js';

	let {
		value = $bindable(),
		people,
		selfId,
		label,
		variant = 'field',
		onchange
	}: {
		value: UserColor;
		/**
		 * Everyone, for the line under the swatches that says who else wears
		 * the chosen color. Advisory: an empty list just says nobody does, and
		 * the picker still works - sharing a color is allowed.
		 */
		people: PersonEntry[];
		/** Whoever this picker edits, left out of the holders; '' for an account not created yet. */
		selfId: string;
		/** Visible above the swatches and the group's accessible name. */
		label: string;
		/**
		 * `field` sits among form fields, label above, left-aligned. `head`
		 * sits centered in a profile head under the name, which already says
		 * what the swatches are: the label is for screen readers only.
		 */
		variant?: 'field' | 'head';
		onchange?: (color: UserColor) => void;
	} = $props();

	const uid = $props.id();

	const holders = $derived(colorHolders(people, value, selfId));
	const head = $derived(variant === 'head');
</script>

<div class="@container space-y-2 {head ? 'w-full text-center' : ''}">
	<span id={uid} class={head ? 'sr-only' : 'block text-caption font-semibold'}>{label}</span>
	<!--
		Explicit 44px tracks rather than `grid-cols-8`: a fraction-sized column
		would stretch with whatever holds the picker and scatter the circles
		across it. Four per row is 212px, so the palette lands as two tidy rows
		inside a 360px card instead of wrapping ragged or scrolling sideways,
		and becomes one row of eight once 436px are there.

		The switch is a container query, not a breakpoint: this picker sits in
		a settings card that is 664px wide on a desktop and in a dialog that is
		384px wide on the same screen, so the viewport cannot answer the only
		question that matters here - how much room the row itself has. The
		head's wider gaps close up to the field's where four would not fit
		with them (a 320px phone), and the same query drops to two per row
		when even that is too wide (200% text: the tracks are in rem).
	-->
	<RadioGroup.Root
		bind:value
		onValueChange={(color) => onchange?.(color as UserColor)}
		aria-labelledby={uid}
		aria-describedby="{uid}-holders"
		class="grid grid-cols-[repeat(4,2.75rem)] @max-[13.25rem]:grid-cols-[repeat(2,2.75rem)] @min-[28rem]:grid-cols-[repeat(8,2.75rem)] {head
			? 'justify-center gap-x-5 gap-y-2 @max-[14.75rem]:gap-x-3 @min-[28rem]:gap-x-2.5'
			: 'gap-3'}"
	>
		{#each USER_COLORS as color (color)}
			<!--
				The item is the 44px hit area; the visible circle inside is 32px.
				The check mark, not the ring, is what makes the choice readable
				without color vision - the ring around a pale swatch is easy to
				miss, a mark inside it is not.

				The selected ring sits on the circle in its own foreground tone;
				the focus outline is `primary` and sits on the hit area around it,
				so a keyboard user sees both at once.

				The inset edge is the foreground at 30%, 50% in dark: at 30% the
				dark swatches met the card at about 2.4:1, at 50% every edge is
				over 3.5:1.
			-->
			<RadioGroup.Item
				value={color}
				aria-label={userColorLabel(color)}
				class="group flex size-11 items-center justify-center rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				{#snippet children({ checked })}
					<span
						class="flex size-8 items-center justify-center rounded-full hover-tint inset-ring ring-current inset-ring-current/30 ring-offset-2 ring-offset-surface transition group-data-[state=checked]:ring-2 dark:inset-ring-current/50 {userColorClasses(
							color
						)}"
					>
						{#if checked}
							<Check class="size-5" aria-hidden="true" />
						{/if}
					</span>
				{/snippet}
			</RadioGroup.Item>
		{/each}
	</RadioGroup.Root>
	<!--
		Who else wears the chosen color, so picking one that is taken reads as
		"you will look like Sofia", not as a mistake. The visible names stop at
		three; screen readers get everyone in full through the sr-only span,
		which is what the group's description reads.
	-->
	<p id="{uid}-holders" class="text-caption text-pretty text-text-muted">
		<span class="font-semibold text-text">{userColorLabel(value)}</span>
		{#if holders.length > 0}
			<!-- Two slots whatever the count, so the line never grows sideways:
			     two faces, or one face and "+N" for the rest. -->
			<FaceStack
				people={holders}
				slots={2}
				size="xs"
				ring="ring-surface"
				class="mx-0.5 align-middle"
			/>
		{/if}
		<span aria-hidden="true">{holdersText(holders)}.</span>
		<span class="sr-only">{holdersFullText(holders)}.</span>
		<!-- Only when there is someone to share with. -->
		{#if holders.length > 0}{m.color_holders_fine()}{/if}
	</p>
</div>
