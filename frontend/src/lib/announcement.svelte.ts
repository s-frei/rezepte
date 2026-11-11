/**
 * A live region's text. A screen reader stays silent when the text repeats,
 * so the same message twice in a row gets a trailing nbsp to be new.
 */
export class Announcement {
	text = $state('');

	say(next: string) {
		this.text = next === this.text ? `${next}\u00A0` : next;
	}
}
