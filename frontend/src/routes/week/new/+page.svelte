<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import X from '@lucide/svelte/icons/x';
	import { getLocalTimeZone, today as todayIn } from '@internationalized/date';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { isSignedOut } from '#lib/api/client.js';
	import { createPlan, type MealPlan } from '#lib/api/mealplan.js';
	import WeekView from '#lib/components/mealplan/WeekView.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import IconButton from '#lib/components/ui/IconButton.svelte';
	import { handKeyboardTo } from '#lib/keyboard.js';
	import { PlanState } from '#lib/mealplan/plan-state.svelte.js';
	import { mondayOf, weekDays } from '#lib/mealplan/week.js';
	import { m } from '#lib/paraglide/messages.js';
	import { rememberPlan, shell } from '#lib/shell.svelte.js';

	// A plan before it exists, as /lists/new is a list: the name as a field
	// at the top, the empty week it will be dimmed below. Only "Create" makes
	// the plan; leaving any other way leaves nothing behind.
	let name = $state('');
	let saving = $state(false);
	let field: HTMLInputElement | undefined = $state();

	const today = todayIn(getLocalTimeZone()).toString();
	const days = weekDays(mondayOf(today));
	const range = { from: days[0], to: days[6] };
	// What the week will look like: nothing planned yet. Never sent anywhere.
	const preview = new PlanState(
		{
			id: '',
			name: null,
			personal: false,
			revision: 0,
			owner: { id: '', displayName: '', color: '', avatarId: null },
			members: [],
			pending: [],
			...range,
			entries: [],
			unlisted: 0,
			lists: []
		} satisfies MealPlan,
		() => {}
	);

	onMount(() => field && handKeyboardTo(field));

	// The draft is the new plan: the bar offers nothing else to make.
	$effect(() => {
		shell.actions = noActions;
		return () => (shell.actions = undefined);
	});

	async function create(event: SubmitEvent) {
		event.preventDefault();
		const trimmed = name.trim();
		if (!trimmed || saving) return;
		saving = true;
		try {
			const plan = await createPlan(trimmed, range);
			rememberPlan(plan.id);
			// Back from the new plan goes where "New plan" was picked, not to the draft.
			await goto(resolve(`/week?plan=${encodeURIComponent(plan.id)}`), {
				replaceState: true,
				state: { created: true }
			});
		} catch (error) {
			if (field) handKeyboardTo(field);
			if (!isSignedOut(error)) toast.error(m.plan_create_error());
		} finally {
			saving = false;
		}
	}

	function cancelOnEscape(event: KeyboardEvent) {
		if (event.key === 'Escape') void goto(resolve('/week'));
	}
</script>

{#snippet noActions()}{/snippet}

<svelte:head><title>{m.plan_new()} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<h1 class="sr-only">{m.plan_new()}</h1>
	<!-- As the list draft: phone, close and "Create" above the name; desktop, both beside it. -->
	<form onsubmit={create} class="mb-6">
		<header class="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-x-4">
			<IconButton
				href={resolve('/week')}
				label={m.common_cancel()}
				class="justify-self-start md:hidden"
			>
				<X class="size-5" aria-hidden="true" />
			</IconButton>
			<div class="col-span-2 row-start-2 mt-5 min-w-0 md:col-span-1 md:row-start-1 md:mt-0">
				<input
					bind:this={field}
					bind:value={name}
					onkeydown={cancelOnEscape}
					aria-label={m.plan_name_label()}
					placeholder={m.plan_new_name_placeholder()}
					autocomplete="off"
					enterkeyhint="done"
					maxlength={80}
					class="w-full min-w-0 border-b-2 border-border bg-transparent pb-1 font-display text-display-md font-medium text-text transition-colors outline-none placeholder:text-text-muted focus:border-primary md:text-display-lg"
				/>
			</div>
			<div class="col-start-2 row-start-1 flex items-center gap-2 md:mb-1">
				<Button variant="secondary" href={resolve('/week')} class="max-md:hidden">
					<X class="size-4" aria-hidden="true" />
					{m.common_cancel()}
				</Button>
				<Button type="submit" busy={saving} disabled={!name.trim()} label={m.plan_new_create()}>
					<Check class="size-4" aria-hidden="true" />
					<span class="md:hidden">{m.plan_new_create_short()}</span>
					<span class="max-md:hidden">{m.plan_new_create()}</span>
				</Button>
			</div>
		</header>
	</form>

	<!-- What the plan will be, waiting until it exists. -->
	<div inert class="opacity-40">
		<WeekView week={preview} {today} onadd={() => {}} />
	</div>
</div>
