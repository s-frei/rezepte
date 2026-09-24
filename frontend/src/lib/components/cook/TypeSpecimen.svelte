<script lang="ts">
	import { Popover, RadioGroup } from 'bits-ui';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fly } from 'svelte/transition';
	import { m } from '$lib/paraglide/messages';
	import {
		DEFAULT_TYPE_SIZE,
		TYPE_SIZE_CLASSES,
		TYPE_SIZES,
		type TypeSize
	} from '$lib/recipe/type-size';

	let {
		value = $bindable(DEFAULT_TYPE_SIZE),
		open = $bindable(false)
	}: {
		value?: TypeSize;
		/** Bound so the page can stand its arrow keys down while the specimen is open. */
		open?: boolean;
	} = $props();

	// Paraglide compiles one function per key, so the names cannot be looked
	// up by string; the compiler has to see every call.
	const NAMES: Record<TypeSize, () => string> = {
		brevier: m.cook_type_size_brevier,
		bourgeois: m.cook_type_size_bourgeois,
		pica: m.cook_type_size_pica,
		'great-primer': m.cook_type_size_great_primer,
		'double-pica': m.cook_type_size_double_pica
	};

	const duration = $derived(prefersReducedMotion.current ? 0 : 180);
</script>

<Popover.Root bind:open>
	<!-- The trigger is itself a small specimen. `text-box` trims the letters'
	     box to cap height over the baseline, so the flex centering centers the
	     ink of the "A" itself rather than a metric box with descender room
	     below it - no em shift, which lands differently per pixel density and
	     per browser's reading of the font's metrics. -->
	<Popover.Trigger
		aria-label={m.cook_type_size()}
		class="inline-flex size-10 shrink-0 items-center justify-center rounded-full border border-border bg-background font-display text-text transition hover:brightness-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
	>
		<span class="inline-flex items-baseline" aria-hidden="true">
			<span class="text-body-lg font-medium [text-box:trim-both_cap_alphabetic]">A</span><span
				class="text-body-sm [text-box:trim-both_cap_alphabetic]">a</span
			>
		</span>
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content forceMount side="bottom" align="end" sideOffset={8} collisionPadding={20}>
			{#snippet child({ wrapperProps, props, open: isOpen })}
				{#if isOpen}
					<div {...wrapperProps} class="z-50">
						<div
							{...props}
							class="w-[min(22rem,calc(100vw-2.5rem))] rounded-2xl bg-surface-elevated px-5 pt-3 pb-4 shadow-dialog outline-none"
							transition:fly={{ y: -8, duration }}
						>
							<!-- A type specimen: every size sits on one shared baseline,
							     drawn as a hairline, so the row reads as the same letters
							     set larger and larger rather than as five buttons. One grid
							     holds letters, rule and marks, so each mark lands under the
							     column of its letters and on the rule, whatever their size. -->
							<RadioGroup.Root
								bind:value
								orientation="horizontal"
								aria-label={m.cook_type_size()}
								class="grid grid-cols-[repeat(5,auto)] items-baseline justify-between"
							>
								{#each TYPE_SIZES as size (size)}
									<RadioGroup.Item
										value={size}
										aria-label={NAMES[size]()}
										class="flex min-h-11 min-w-11 items-baseline justify-center rounded-sm px-1 pb-2 font-display leading-none transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary data-[state=checked]:text-text data-[state=unchecked]:text-text-muted data-[state=unchecked]:hover:text-text {TYPE_SIZE_CLASSES[
											size
										]}"
									>
										<span aria-hidden="true">Aa</span>
									</RadioGroup.Item>
								{/each}
								<span class="col-span-5 border-t border-border" aria-hidden="true"></span>
								{#each TYPE_SIZES as size (size)}
									<span
										aria-hidden="true"
										class="-mt-[3px] h-[5px] w-6 justify-self-center rounded-pill bg-primary transition-opacity {size ===
										value
											? 'opacity-100'
											: 'opacity-0'}"
									></span>
								{/each}
							</RadioGroup.Root>
							<p class="mt-3 text-center font-display text-body text-text-muted italic">
								{NAMES[value]()}
							</p>
						</div>
					</div>
				{/if}
			{/snippet}
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
