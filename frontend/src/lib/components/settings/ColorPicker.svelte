<script lang="ts">
	import { RadioGroup } from 'bits-ui';
	import Check from '@lucide/svelte/icons/check';
	import type { ColorUsage } from '$lib/api/auth';
	import { m } from '$lib/paraglide/messages';
	import { USER_COLORS, userColorClasses, type UserColor } from '$lib/user/color';

	let {
		value = $bindable(),
		usage,
		label
	}: {
		value: UserColor;
		/**
		 * How many accounts hold each color. Advisory: an empty list marks
		 * nothing and the picker still works, because the counts are a hint
		 * about who you will look like, never a rule about what you may pick.
		 */
		usage: ColorUsage[];
		/** Visible above the swatches and the group's accessible name. */
		label: string;
	} = $props();

	const uid = $props.id();

	// Paraglide compiles one function per key, so the color names cannot be
	// looked up as `m['user_color_' + color]` - the compiler has to see every
	// call. Written out here rather than in $lib/user/color.ts: that module is
	// the class table, and this is the only place a color is ever named.
	const COLOR_LABELS: Record<UserColor, () => string> = {
		amber: m.user_color_amber,
		clay: m.user_color_clay,
		rose: m.user_color_rose,
		plum: m.user_color_plum,
		sage: m.user_color_sage,
		olive: m.user_color_olive,
		teal: m.user_color_teal,
		slate: m.user_color_slate
	};

	const counts = $derived(new Map(usage.map((entry) => [entry.color, entry.count])));

	/**
	 * A color held by somebody else. The selected one is never marked: its
	 * count includes whoever this picker is editing, so marking it would tell
	 * every person their own color is taken - by themselves.
	 */
	const taken = $derived(
		new Set(USER_COLORS.filter((color) => color !== value && (counts.get(color) ?? 0) > 0))
	);
</script>

<div class="@container space-y-2">
	<span id={uid} class="block text-caption font-semibold">{label}</span>
	<!--
		Explicit 44px tracks rather than `grid-cols-8`: a fraction-sized column
		would stretch with whatever holds the picker and scatter the circles
		across it. Four per row is 212px, so the palette lands as two tidy rows
		inside a 360px card instead of wrapping ragged or scrolling sideways,
		and becomes one row of eight once 436px are there.

		The switch is a container query, not a breakpoint: this picker sits in
		a settings card that is 664px wide on a desktop and in a dialog that is
		384px wide on the same screen, so the viewport cannot answer the only
		question that matters here - how much room the row itself has.
	-->
	<RadioGroup.Root
		bind:value
		aria-labelledby={uid}
		class="grid grid-cols-[repeat(4,2.75rem)] gap-3 @min-[28rem]:grid-cols-[repeat(8,2.75rem)]"
	>
		{#each USER_COLORS as color (color)}
			<!--
				`aria-label` carries the color's name alone and the "taken" note
				arrives as a description, so the swatch keeps the name a person
				would say out loud. The check mark, not the ring, is what makes
				the choice readable without color vision - the ring around a
				pale swatch is easy to miss, a mark inside it is not.

				The selected ring is ink, the focus ring is `primary`, and both
				sit at the same offset: arrow keys move selection along with
				focus, so in a radio group the focused swatch is almost always
				the selected one. Painted over each other the ring simply turns
				orange when the group has the keyboard, which is the one thing
				two identical rings could not have said.
			-->
			<RadioGroup.Item
				value={color}
				aria-label={COLOR_LABELS[color]()}
				aria-describedby={taken.has(color) ? `${uid}-taken` : undefined}
				class="flex size-11 items-center justify-center rounded-full transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary data-[state=checked]:ring-2 data-[state=checked]:ring-text data-[state=checked]:ring-offset-2 data-[state=checked]:ring-offset-surface {userColorClasses(
					color
				)}"
			>
				{#snippet children({ checked })}
					{#if checked}
						<Check class="size-5" aria-hidden="true" />
					{:else if taken.has(color)}
						<!-- `bg-current` is the swatch's own foreground token, so the
						     dot keeps its measured contrast on every color. -->
						<span aria-hidden="true" class="size-1.5 rounded-full bg-current"></span>
					{/if}
				{/snippet}
			</RadioGroup.Item>
		{/each}
	</RadioGroup.Root>
	{#if taken.size > 0}
		<p id="{uid}-taken" class="flex items-center gap-1.5 text-micro text-text-muted">
			<span aria-hidden="true" class="size-1.5 rounded-full bg-text-muted"></span>
			{m.settings_profile_color_taken()}
		</p>
	{/if}
</div>
