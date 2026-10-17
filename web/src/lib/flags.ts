// web/src/lib/flags.ts
//
// Feature flags. PUBLIC_FLAG_<NAME>=1 turns a feature on; anything else is
// off. Read at runtime like the CARTO key, so flipping one is a restart, not a
// rebuild (`PUBLIC_FLAG_X=1 npm run dev`, or the web service's environment in
// compose.yml). Add a flag: one line below, one row in the CLAUDE.md env table.
import { env } from '$env/dynamic/public';

const on = (name: string) => env[`PUBLIC_FLAG_${name}`] === '1';

export const flags = {
  /** Curated routes: map layer, peek bar, list, featured picks, saved-route rows. */
  curatedRoutes: on('CURATED_ROUTES')
};
