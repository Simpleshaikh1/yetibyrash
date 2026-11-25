// Line icons: 24px box, 1.6px stroke, coloured by the surrounding text (design system › Iconography).
const paths = {
  menu: "M4 7h16M4 12h16M4 17h10",
  bag: "M5 8h14l-1 12H6L5 8Z M9 8a3 3 0 0 1 6 0",
  back: "M15 5 8 12l7 7",
  search: "M11 4.5a6.5 6.5 0 1 1 0 13 6.5 6.5 0 0 1 0-13Z M16 16l4 4",
  home: "M4 11 12 4l8 7v9h-5v-6H9v6H4z",
  grid: "M4 4h7v7H4z M13 4h7v7h-7z M4 13h7v7H4z M13 13h7v7h-7z",
  heart: "M12 20s-7-4.4-7-10a4 4 0 0 1 7-2.6A4 4 0 0 1 19 10c0 5.6-7 10-7 10Z",
  track: "M5 10a2 2 0 1 1 0 4 2 2 0 0 1 0-4Z M12 10a2 2 0 1 1 0 4 2 2 0 0 1 0-4Z M19 10a2 2 0 1 1 0 4 2 2 0 0 1 0-4Z M7 12h3M14 12h3",
  filter: "M4 6h16M7 12h10M10 18h4",
  close: "M6 6l12 12M18 6 6 18",
} as const;

export type IconName = keyof typeof paths;

export function Icon({ name, size = 22 }: { name: IconName; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d={paths[name]} />
    </svg>
  );
}
