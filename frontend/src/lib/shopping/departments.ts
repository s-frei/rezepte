import { m } from '#lib/paraglide/messages.js';
import type { Department } from '#lib/api/shopping.js';

/** Aisle order of a shopping trip. */
export const DEPARTMENTS: Department[] = [
	'produce',
	'bakery',
	'chilled',
	'meat',
	'pantry',
	'frozen',
	'drinks',
	'household',
	'other'
];

export function departmentLabel(d: Department): string {
	return m[`shopping_department_${d}`]();
}
