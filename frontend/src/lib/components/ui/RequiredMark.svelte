<script lang="ts" module>
	export type RequiredState = 'empty' | 'filled' | 'error';

	/** The error wins: a filled field can still be wrong, and then it is the error that matters. */
	export function requiredState(filled: boolean, error: boolean): RequiredState {
		return error ? 'error' : filled ? 'filled' : 'empty';
	}
</script>

<script lang="ts">
	let { state }: { state: RequiredState } = $props();

	// Amber while there is still something to fill in, muted once there is
	// not, red only after a save found a problem - never red before the
	// person has done anything, which would read as an error they had not
	// made yet.
	const tone = { empty: 'text-primary', filled: 'text-text-muted', error: 'text-destructive' };
</script>

<!-- Hidden from assistive tech: the field carries `aria-required`, and a
     screen reader reading "star" after every label would add nothing. -->
<span
	aria-hidden="true"
	data-required-mark
	data-state={state}
	class="transition-colors motion-reduce:transition-none {tone[state]}">*</span
>
