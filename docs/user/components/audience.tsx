// Who a page or a section is for, when that is not everyone: a small word
// badge beside the page's title or the section's heading. A page sets
// `audience` in its frontmatter; a section puts <Audience role="…" /> into
// its heading. The sidebar stays unmarked - a badge there beside one entry
// among many reads as noise, not as a hint.

export type Role = 'admin' | 'owner';

const BADGE: Record<Role, string> = {
	admin: 'Admins',
	owner: 'Owner'
};

export const ROLE_LABEL: Record<Role, string> = {
	admin: 'Admins only',
	owner: 'Owner only'
};

const ROLE_DESCRIPTION: Record<Role, string> = {
	admin: 'for admins, the instance owner included',
	owner: 'for the instance owner alone, the account created on the very first start'
};

/** The badge: its word on the page, its full meaning as tooltip and for screen readers. */
export function Audience({ role, className = 'ms-[0.5em]' }: { role: Role; className?: string }) {
	return (
		<span
			className={`inline-block rounded-full border border-fd-primary/30 bg-fd-primary/10 px-2 py-0.5 align-middle font-sans text-[0.7rem] leading-none font-semibold tracking-wide text-fd-primary uppercase ${className}`}
			title={ROLE_LABEL[role]}
		>
			<span aria-hidden="true">{BADGE[role]}</span>
			<span className="sr-only">{ROLE_LABEL[role]}</span>
		</span>
	);
}

/** The key to the badges, shown once on the guide's index page. */
export function AudienceLegend() {
	return (
		<dl className="not-prose my-6 grid grid-cols-[auto_1fr] items-baseline gap-x-3 gap-y-2 text-sm">
			{(Object.keys(ROLE_LABEL) as Role[]).map((role) => (
				<div key={role} className="contents">
					<dt>
						<Audience role={role} className="" />
					</dt>
					<dd>
						{ROLE_LABEL[role]} — {ROLE_DESCRIPTION[role]}
					</dd>
				</div>
			))}
		</dl>
	);
}
