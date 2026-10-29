<script lang="ts">
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade } from 'svelte/transition';
	import { m } from '#lib/paraglide/messages.js';
	import {
		estimateStrength,
		strengthLabel,
		type Strength
	} from '#lib/settings/password-strength.js';

	// Advisory only: nothing here blocks a submit. The API's length rule is
	// the one rule a password has to pass, and a check that lived only in the
	// browser would be one curl call away from not existing.
	let { password, userInputs = [] }: { password: string; userInputs?: string[] } = $props();

	let strength = $state<Strength | null>(null);

	// Waits for a pause in typing, so the label - a live region - is announced
	// once per word typed rather than once per character, and a result that
	// arrives after the field changed again is dropped rather than shown.
	$effect(() => {
		const value = password;
		const inputs = [...userInputs];
		if (value === '') {
			strength = null;
			return;
		}
		let current = true;
		const timer = setTimeout(async () => {
			const result = await estimateStrength(value, inputs);
			if (current) strength = result;
		}, 250);
		return () => {
			current = false;
			clearTimeout(timer);
		};
	});

	const fill = $derived(
		strength === null
			? ''
			: strength <= 1
				? 'bg-destructive'
				: strength === 2
					? 'bg-primary'
					: 'bg-success'
	);
	// Segments filled: a score of 0 still lights one, so "very weak" reads as
	// a measurement and not as a meter that has not started yet.
	const filled = $derived(strength === null ? 0 : Math.max(1, strength));
	// The meter fades in with the first character; the segments then take
	// their color one after another from the left, so a new score reads as
	// the meter filling up rather than a blink.
	const appear = $derived(prefersReducedMotion.current ? 0 : 160);
</script>

{#if password !== ''}
	<div class="space-y-1" in:fade={{ duration: appear }}>
		<div class="grid grid-cols-4 gap-1" aria-hidden="true">
			{#each [1, 2, 3, 4] as segment (segment)}
				<span
					class="h-1 rounded-pill transition-colors duration-200 ease-glide motion-reduce:transition-none {segment <=
					filled
						? fill
						: 'bg-border'}"
					style:transition-delay="{(segment - 1) * 40}ms"
				></span>
			{/each}
		</div>
		<p
			aria-live="polite"
			class="text-micro {strength !== null && strength <= 1
				? 'font-medium text-destructive'
				: 'text-text-muted'}"
		>
			{strength === null ? '' : m.settings_password_strength({ level: strengthLabel(strength) })}
		</p>
	</div>
{/if}
