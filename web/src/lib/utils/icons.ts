// web/src/lib/utils/icons.ts
//
// The stroked 24-unit icon set used by the explore chrome (Penpot "Home
// rework"). Every icon is drawn in currentColor so the caller's text token
// colours it; the stroke width is the Penpot value (1.75 on chrome, 1.6 in
// flow panels, 2.2 on markers).
export const ICONS = {
  search: '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5"/>',
  menu: '<path d="M4 7h16M4 12h16M4 17h16"/>',
  layers: '<path d="M12 3l9 5-9 5-9-5 9-5z"/><path d="M3 13l9 5 9-5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  minus: '<path d="M5 12h14"/>',
  locate: '<circle cx="12" cy="12" r="4"/><path d="M12 2v4M12 18v4M2 12h4M18 12h4"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  chevronUp: '<path d="M6 15l6-6 6 6"/>',
  chevronDown: '<path d="M6 9l6 6 6-6"/>',
  chevronLeft: '<path d="M15 6l-6 6 6 6"/>',
  chevronRight: '<path d="M9 6l6 6-6 6"/>',
  route:
    '<circle cx="6" cy="18" r="2"/><circle cx="18" cy="6" r="2"/><path d="M8 18h5a3 3 0 0 0 0-6h-2a3 3 0 0 1 0-6h5"/>',
  sparkle: '<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8L12 3z"/>',
  history: '<path d="M3 12a9 9 0 1 0 3-6.7"/><path d="M3 4v5h5"/><path d="M12 7v5l3 2"/>',
  bookmark: '<path d="M6 3h12v18l-6-4-6 4V3z"/>',
  x: '<path d="M6 6l12 12M18 6L6 18"/>',
  bicycle:
    '<circle cx="5.5" cy="17" r="3.5"/><circle cx="18.5" cy="17" r="3.5"/>' +
    '<path d="M5.5 17l3.5-8h5.5l4 8M9 9L7.5 6h3M14.5 9l1.5 8"/>',
  flag: '<path d="M6 21V4h11l-2.5 4 2.5 4H6"/>',
  share: '<path d="M12 3v12M8 7l4-4 4 4"/><path d="M5 12v7h14v-7"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 8v.5M12 11v5"/>',
  sunHorizon:
    '<path d="M3 17h18M6 13a6 6 0 0 1 12 0"/><path d="M12 5v2M4.5 8l1.5 1.5M19.5 8L18 9.5"/>',
  wind: '<path d="M3 8h10a3 3 0 1 0-3-3"/><path d="M3 12h15a3 3 0 1 1-3 3"/><path d="M3 16h7a2 2 0 1 1-2 2"/>',
  rain: '<path d="M7 15a4.5 4.5 0 0 1-.5-9A6 6 0 0 1 18 7a4 4 0 0 1 0 8H7z"/><path d="M9 19v2M13 19v2"/>',
  grip: '<path d="M9 5v14M15 5v14"/>',
  pencil: '<path d="M4 20l4-1 10-10-3-3L5 16l-1 4z"/><path d="M13 7l3 3"/>',
  pin: '<path d="M12 21s-7-6.5-7-11a7 7 0 0 1 14 0c0 4.5-7 11-7 11z"/><circle cx="12" cy="10" r="2.5"/>',
  check: '<path d="M5 12l5 5 9-10"/>',
  swap: '<path d="M7 4v16M7 20l-3-3M7 20l3-3"/><path d="M17 20V4M17 4l-3 3M17 4l3 3"/>',
  konbini:
    '<path d="M4 9.5L5.5 5h13L20 9.5M4 9.5V20h16V9.5"/>' +
    '<path d="M4 9.5a2.67 2.67 0 0 0 5.33 0 2.67 2.67 0 0 0 5.34 0A2.67 2.67 0 0 0 20 9.5M10 20v-6h4v6"/>',
  cafe: '<path d="M5 9h11v5a5.5 5.5 0 0 1-11 0V9zM16 10h1.5a2 2 0 0 1 0 4H16M4 20h14"/>',
  water: '<path d="M12 3s-6 6.8-6 11a6 6 0 0 0 12 0c0-4.2-6-11-6-11z"/>',
  repair:
    '<path d="M14.5 3.5a5 5 0 0 0-5.3 6.9L3 16.6 6.4 20l6.2-6.2a5 5 0 0 0 6.9-5.3l-2.8 2.8-2.7-.7-.7-2.7 2.8-2.8z"/>',
  zoomOut: '<circle cx="11" cy="11" r="7"/><path d="M20 20l-3.5-3.5M8 11h6"/>'
} as const;

export type IconName = keyof typeof ICONS;

export function iconSvg(name: IconName, strokeWidth = 1.75): string {
  return (
    `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="${strokeWidth}" ` +
    `stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name]}</svg>`
  );
}
