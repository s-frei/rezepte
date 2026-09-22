import { setAriaStrings } from 'svelte-dnd-action';
import { m } from '$lib/paraglide/messages';

/**
 * svelte-dnd-action speaks to screen readers itself, out of its own built-in
 * strings. This hands it the app's copy instead, in whichever language the
 * page is rendering.
 *
 * The setting is global to the document and applies to every dnd zone, so it
 * only has to happen once - `applied` keeps repeated editor mounts from
 * redoing the work. `setAriaStrings` is a no-op on the server, so importing
 * this module is safe either way.
 */
let applied = false;

const KEY_PHRASES = {
	space: () => m.dnd_key_space(),
	enter: () => m.dnd_key_enter(),
	space_or_enter: () => m.dnd_key_space_or_enter()
};

export function applyDndAriaStrings(): void {
	if (applied) {
		return;
	}
	applied = true;
	setAriaStrings({
		// Two full sentences rather than one plus an appended clause: German
		// word order doesn't survive gluing message fragments together.
		dragStarted: ({ itemLabel, zoneLabel, canMoveBetweenZones }) =>
			canMoveBetweenZones
				? m.dnd_drag_started_multi({ item: itemLabel, zone: zoneLabel })
				: m.dnd_drag_started({ item: itemLabel, zone: zoneLabel }),
		movedToPosition: ({ itemLabel, zoneLabel, position, count }) =>
			m.dnd_moved_to_position({ item: itemLabel, zone: zoneLabel, position, count }),
		movedToZoneEnd: ({ itemLabel, zoneLabel }) =>
			m.dnd_moved_to_zone_end({ item: itemLabel, zone: zoneLabel }),
		movedToZoneStart: ({ itemLabel, zoneLabel }) =>
			m.dnd_moved_to_zone_start({ item: itemLabel, zone: zoneLabel }),
		dropped: ({ itemLabel, zoneLabel, position, count }) =>
			m.dnd_dropped({ item: itemLabel, zone: zoneLabel, position, count }),
		zoneActiveInstruction: ({ keyboardDragTrigger }) =>
			m.dnd_zone_instruction({ key: KEY_PHRASES[keyboardDragTrigger]() }),
		zoneDragDisabledInstruction: m.dnd_zone_disabled_instruction()
	});
}
