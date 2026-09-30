// web/src/lib/utils/format.ts
//
// Text formatting shared by the explore panels. Copy follows the Penpot
// frames ("24.1 km", "↗ 118 m", "1 h 25 m", "Today, 10:00").

export function km(metres: number, digits = 1): string {
  return metres >= 1000 ? `${(metres / 1000).toFixed(digits)} km` : `${Math.round(metres)} m`;
}

export function climb(metres: number): string {
  return `↗ ${Math.round(metres)} m`;
}

/** "1 h 25 m" / "32 min". */
export function duration(seconds: number): string {
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min`;
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return m === 0 ? `${h} h` : `${h} h ${m} m`;
}

export function hhmm(date: Date): string {
  return date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false });
}

/** "Today, 10:00" · "Tomorrow, 03:00" · "Sat 11:00". */
export function dayTime(date: Date, now = new Date()): string {
  const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const diff = Math.round((day(date) - day(now)) / 86_400_000);
  if (diff === 0) return `Today, ${hhmm(date)}`;
  if (diff === 1) return `Tomorrow, ${hhmm(date)}`;
  return `${date.toLocaleDateString('en-GB', { weekday: 'short' })} ${hhmm(date)}`;
}

/** Whether `iso` falls in the current hour, i.e. the chip should say "Now". */
export function isNow(iso: string, now = new Date()): boolean {
  const d = new Date(iso);
  return Math.floor(d.getTime() / 3_600_000) === Math.floor(now.getTime() / 3_600_000);
}

const COMPASS = ['N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];
/** Wind blows *from* `bearingDeg`; "light NE wind" per the Penpot chip. */
export function windPhrase(speedMs: number, bearingDeg: number): string {
  if (speedMs < 1) return 'calm';
  const dir = COMPASS[Math.round(((bearingDeg % 360) + 360) / 45) % 8];
  const strength = speedMs < 3.5 ? 'light' : speedMs < 8 ? 'moderate' : 'strong';
  return `${strength} ${dir} wind`;
}

/** "dry" · "0.4 mm/h drizzle" · "2.3 mm/h rain" · "heavy rain". */
export function rainPhrase(mmh: number): string {
  if (mmh < 0.05) return 'dry';
  if (mmh < 0.5) return `${mmh.toFixed(1)} mm/h drizzle`;
  if (mmh < 8) return `${mmh.toFixed(1)} mm/h rain`;
  return 'heavy rain';
}
