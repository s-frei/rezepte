import type { User } from './api/auth';

/** Current login state, shared across the app. Mutate `session.user`. */
export const session = $state<{ user: User | null }>({ user: null });
