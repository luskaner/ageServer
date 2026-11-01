import { createTheme, type CSSVariablesResolver, type MantineColorsTuple } from '@mantine/core';

// The interface follows the desktop it runs on: `main.tsx` reads the accent
// colour the system highlights controls with and hands it to `createAppTheme`.
// This is the colour that is kept when the desktop offers none (the
// ageServer brand colour).
const DEFAULT_ACCENT = '#60cdff';

// WinUI-style font stack used across the configurator (kept from the Fluent era).
const FONT_FAMILY =
    '"Segoe UI Variable Text", "Segoe UI", -apple-system, BlinkMacSystemFont, Roboto, sans-serif';

// Shade 6 is the one filled controls take their colour from, so the desktop
// accent is kept there exactly. Mantine draws the light variant of a control
// (the selected item of a navbar, for instance) as shade 9 at half brightness
// in the dark scheme, so the dark end of the ramp stays well clear of black
// instead of turning the accent into a grey slab.
const FILLED_SHADE = 6;
const LIGHTEST = 0.97;
const DARKEST = 0.36;

/** Hue, saturation and lightness of a #rrggbb colour, each between 0 and 1. */
function toHsl(hex: string): { h: number; s: number; l: number } {
    const value = Number.parseInt(hex.slice(1), 16);
    const red = ((value >> 16) & 0xff) / 255;
    const green = ((value >> 8) & 0xff) / 255;
    const blue = (value & 0xff) / 255;

    const max = Math.max(red, green, blue);
    const min = Math.min(red, green, blue);
    const l = (max + min) / 2;
    if (max === min) return { h: 0, s: 0, l };

    const delta = max - min;
    const s = l > 0.5 ? delta / (2 - max - min) : delta / (max + min);
    let h: number;
    if (max === red) h = ((green - blue) / delta + (green < blue ? 6 : 0)) / 6;
    else if (max === green) h = ((blue - red) / delta + 2) / 6;
    else h = ((red - green) / delta + 4) / 6;
    return { h, s, l };
}

/** #rrggbb colour for a hue, saturation and lightness, each between 0 and 1. */
function fromHsl(h: number, s: number, l: number): string {
    const channel = (offset: number) => {
        const k = (offset + h * 12) % 12;
        const a = s * Math.min(l, 1 - l);
        const value = l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
        return Math.round(value * 255)
            .toString(16)
            .padStart(2, '0');
    };
    return `#${channel(0)}${channel(8)}${channel(4)}`;
}

/**
 * Mantine wants ten shades per colour. The desktop accent is kept at the shade
 * filled controls use, and the remaining shades walk towards white and black
 * without leaving its hue, so light variants, hover states and disabled colours
 * all stay in the colour the system chose.
 */
function accentRamp(accent: string): MantineColorsTuple {
    const { h, s, l } = toHsl(accent);
    // Keep the ramp monotonic for accents that are already very light or very
    // dark by themselves.
    const lightest = Math.max(LIGHTEST, l);
    const darkest = Math.min(DARKEST, l * 0.8);
    const lighter = (step: number) =>
        fromHsl(h, s, lightest - (lightest - l) * (step / FILLED_SHADE));
    const darker = (step: number) => fromHsl(h, s, l - (l - darkest) * (step / (9 - FILLED_SHADE)));
    return [
        lighter(0),
        lighter(1),
        lighter(2),
        lighter(3),
        lighter(4),
        lighter(5),
        fromHsl(h, s, l),
        darker(1),
        darker(2),
        darker(3),
    ];
}

/**
 * Build the application theme around an accent colour, which is the desktop's
 * own when it has one. Anything that is not a #rrggbb colour falls back to the
 * brand accent.
 */
export function createAppTheme(accent: string = DEFAULT_ACCENT) {
    return createTheme({
        primaryColor: 'accent',
        primaryShade: FILLED_SHADE,
        // Pick black or white text automatically on the accent filled variants.
        autoContrast: true,
        colors: { accent: accentRamp(/^#[0-9a-f]{6}$/i.test(accent) ? accent : DEFAULT_ACCENT) },
        fontFamily: FONT_FAMILY,
        headings: { fontFamily: FONT_FAMILY },
        // WinUI 3 rounds controls, inputs and settings cards with the same
        // 4 px (ControlCornerRadius); only flyouts and dialogs use 8 px.
        defaultRadius: 'sm',
        cursorType: 'pointer',
        // Leave animations out when the desktop asks for reduced motion.
        respectReducedMotion: true,
    });
}

// The Wails 3 window uses a native Mica/Acrylic backdrop; keep the webview body
// transparent so the effect stays visible behind the app (previously done with a
// transparent Fluent theme override). The scheme itself follows the desktop, so
// both the light and the dark set have to stay transparent. Error states use a
// muted garnet red instead of Mantine's vivid default `#fa5252`.
export const cssVariablesResolver: CSSVariablesResolver = () => ({
    variables: {},
    light: {
        '--mantine-color-body': 'transparent',
        '--mantine-color-error': '#9c2b2b',
        '--mantine-color-warning': '#8a6100',
    },
    dark: {
        '--mantine-color-body': 'transparent',
        '--mantine-color-error': '#c05a54',
        '--mantine-color-warning': '#d9a13b',
    },
});
