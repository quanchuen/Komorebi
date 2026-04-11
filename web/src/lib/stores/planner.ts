// web/src/lib/stores/planner.ts
import { writable } from 'svelte/store';
import type { RoutePlan, DirectionsResponse, RoutingPreferences } from '$lib/api/types';

export interface PlannerStop {
  id: string; // local UUID before plan is created
  lat: number;
  lon: number;
  label: string;
}

export const plannerStops = writable<PlannerStop[]>([]);
export const plannerPreferences = writable<RoutingPreferences>({ shade: 0.5, greenery: 0.5, wind: 0.5 });
export const plannerResult = writable<DirectionsResponse | null>(null);
export const plannerPlan = writable<RoutePlan | null>(null);
export const plannerLoading = writable<boolean>(false);
export const plannerError = writable<string | null>(null);
export const plannerTaskInput = writable<string>('');
