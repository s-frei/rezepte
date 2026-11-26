<script lang="ts">
	import Download from '@lucide/svelte/icons/download';
	import { Dialog } from 'bits-ui';
	import Button from '#lib/components/ui/Button.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import SegmentedControl from '#lib/components/ui/SegmentedControl.svelte';
	import Switch from '#lib/components/ui/Switch.svelte';
	import ServingsStepper from '#lib/components/recipe/ServingsStepper.svelte';
	import { printOptions, setPrintOption } from '#lib/recipe/print-options.svelte.js';
	import { narrow } from '#lib/ui/narrow.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * "Als PDF" from the Export card: how the ticked recipes become PDFs. Every
	 * choice is remembered the moment it is made (`printOptions`, shared with
	 * printing a recipe); "Create PDF" closes the dialog into the card's
	 * progress on its button.
	 */
	let {
		open = $bindable(false),
		count,
		oncreate
	}: {
		open?: boolean;
		/** How many recipes are ticked. */
		count: number;
		oncreate: () => void;
	} = $props();

	// Adjust starts at two, a household's usual count; the override is the
	// remembered value, so no number survives a switch back to "As in recipe".
	const ADJUST_DEFAULT = 2;
	const short = $derived(narrow.current);
	const row =
		'flex min-h-12 items-center justify-between gap-3 px-4 py-2 text-body-sm font-semibold';

	function create() {
		open = false;
		oncreate();
	}
</script>

{#snippet body()}
	<p class="text-label font-bold tracking-[0.08em] text-text-muted uppercase">
		{count === 1 ? m.settings_transfer_pdf_count_one() : m.settings_transfer_pdf_count({ count })}
	</p>
	<Dialog.Title class="mt-1 font-display text-heading font-medium">
		{m.settings_transfer_pdf_title()}
	</Dialog.Title>
	<div class="mt-4 divide-y divide-border rounded-lg border border-border bg-surface">
		<div class={row}>
			<span>{m.settings_transfer_pdf_bundle()}</span>
			<!-- Below md the control is an inline-size container, 0 wide unless given a width. -->
			<div class="max-md:max-w-52 max-md:flex-1">
				<SegmentedControl
					label={m.settings_transfer_pdf_bundle()}
					value={printOptions.bundle}
					onchange={(v) => setPrintOption('bundle', v)}
					options={[
						{ value: 'book', label: m.settings_transfer_pdf_bundle_book() },
						{
							value: 'zip',
							label: short
								? m.settings_transfer_pdf_bundle_zip_short()
								: m.settings_transfer_pdf_bundle_zip()
						}
					]}
				/>
			</div>
		</div>
		<div class={row}>
			<span>{m.settings_transfer_pdf_format()}</span>
			<div class="max-md:max-w-52 max-md:flex-1">
				<SegmentedControl
					label={m.settings_transfer_pdf_format()}
					value={printOptions.format}
					onchange={(v) => setPrintOption('format', v)}
					options={[
						{
							value: 'a4',
							label: short
								? m.settings_transfer_pdf_format_a4_short()
								: m.settings_transfer_pdf_format_a4()
						},
						{
							value: 'a5',
							label: short
								? m.settings_transfer_pdf_format_a5_short()
								: m.settings_transfer_pdf_format_a5()
						}
					]}
				/>
			</div>
		</div>
		<!-- On a phone the label sits above a full-width control. -->
		<div class="{row} max-md:flex-col max-md:items-stretch">
			<span>{m.settings_transfer_pdf_servings()}</span>
			<SegmentedControl
				label={m.settings_transfer_pdf_servings()}
				value={printOptions.servings === null ? 'recipe' : 'adjust'}
				onchange={(v) => setPrintOption('servings', v === 'recipe' ? null : ADJUST_DEFAULT)}
				options={[
					{ value: 'recipe', label: m.settings_transfer_pdf_servings_recipe() },
					{ value: 'adjust', label: m.settings_transfer_pdf_servings_adjust() }
				]}
			/>
		</div>
		{#if printOptions.servings !== null}
			<div class="{row} font-normal text-text-muted">
				<span>
					{short
						? m.settings_transfer_pdf_servings_all_short()
						: m.settings_transfer_pdf_servings_all()}
				</span>
				<ServingsStepper
					value={printOptions.servings}
					onchange={(n) => setPrintOption('servings', n)}
				/>
			</div>
		{/if}
		<div class={row}>
			<span>{m.settings_transfer_pdf_photo()}</span>
			<Switch
				label={m.settings_transfer_pdf_photo()}
				checked={printOptions.photo}
				onchange={(on) => setPrintOption('photo', on)}
			/>
		</div>
		<div class={row}>
			<span>{m.settings_transfer_pdf_footer()}</span>
			<Switch
				label={m.settings_transfer_pdf_footer()}
				checked={printOptions.footer}
				onchange={(on) => setPrintOption('footer', on)}
			/>
		</div>
	</div>
{/snippet}

{#snippet footer()}
	<!-- The sheet closes by its handle or a swipe; the dialog needs its Cancel. -->
	{#if !short}
		<Button variant="secondary" onclick={() => (open = false)}>{m.common_cancel()}</Button>
	{/if}
	<Button class="grow md:grow-0" onclick={create}>
		<Download class="size-4" aria-hidden="true" />
		{m.settings_transfer_pdf_create()}
	</Button>
{/snippet}

<ResponsiveDialog bind:open closeLabel={m.common_close()} {body} {footer} />
