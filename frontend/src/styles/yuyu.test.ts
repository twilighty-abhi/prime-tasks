// prime-tasks Y1: checks the Yuyu theme layer without a browser. It reads
// custom-properties/yuyu.scss, resolves the custom properties for each theme the way the
// cascade does, then checks the grey scale and the contrast of every text/background pair.
import {readFileSync} from 'node:fs'
import {resolve} from 'node:path'
import {describe, expect, it} from 'vitest'

const yuyuScss = readFileSync(resolve(__dirname, 'custom-properties/yuyu.scss'), 'utf8')

type Theme = 'light' | 'dark'
type Vars = Record<string, string>

// :root is (0,1,0) and :root.dark is (0,2,0), so in dark mode every declaration made for
// :root.dark wins over a plain :root one regardless of source order.
function resolveTheme(source: string, theme: Theme): Vars {
	const withoutComments = source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/\/\/[^\n]*/g, '')
	const base: Vars = {}
	const dark: Vars = {}
	for (const block of withoutComments.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
		const selectors = block[1]!.split(',').map(selector => selector.trim())
		const target = selectors.includes(':root.dark')
			? (theme === 'dark' ? dark : null)
			: null
		const appliesToRoot = selectors.includes(':root')
		for (const declaration of block[2]!.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) {
			const [, name, value] = declaration
			if (target) target[name!] = value!.trim()
			else if (appliesToRoot) base[name!] = value!.trim()
		}
	}
	return {...base, ...dark}
}

function resolveValue(vars: Vars, value: string, depth = 0): string {
	if (depth > 10) throw new Error(`var() cycle at ${value}`)
	return value.replace(/var\((--[\w-]+)\)/g, (_, name: string) => {
		const next = vars[name]
		if (next === undefined) throw new Error(`${name} is not defined`)
		return resolveValue(vars, next, depth + 1)
	})
}

function hslToHex(h: number, s: number, l: number): string {
	s /= 100
	l /= 100
	const k = (n: number) => (n + h / 30) % 12
	const a = s * Math.min(l, 1 - l)
	const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)))
	return '#' + [f(0), f(8), f(4)].map(x => Math.round(x * 255).toString(16).padStart(2, '0')).join('').toUpperCase()
}

// A colour: a token or variable that resolves to hex, or a Bulma h/s/l triple (--danger etc.).
function color(vars: Vars, name: string): string {
	if (name.startsWith('#')) return name.toUpperCase()
	if (vars[`${name}-h`] !== undefined && vars[name] === undefined) {
		const part = (suffix: string) => parseFloat(resolveValue(vars, vars[`${name}-${suffix}`]!))
		return hslToHex(part('h'), part('s'), part('l'))
	}
	const value = resolveValue(vars, vars[name] ?? (() => { throw new Error(`${name} is not defined`) })())
	if (!/^#[0-9a-f]{6}$/i.test(value)) throw new Error(`${name} resolves to ${value}, not a hex colour`)
	return value.toUpperCase()
}

function luminance(hex: string): number {
	const [r, g, b] = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255).map(c =>
		c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
	return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!
}

export function contrast(a: string, b: string): number {
	const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
	return (hi! + 0.05) / (lo! + 0.05)
}

const themes: Theme[] = ['light', 'dark']

const expectedGreys: Record<Theme, Record<string, string>> = {
	light: {
		'--grey-50': '#FAFAF9', // bg-subtle
		'--grey-100': '#F4F4F2', // bg-muted
		'--grey-200': '#E7E6E2', // border
		'--grey-300': '#D4D3CE', // border-strong
		'--grey-400': '#5F5E5A', // text-muted
		'--grey-500': '#4F4F50', // interpolated
		'--grey-600': '#3F3F46', // text-secondary
		'--grey-700': '#2C2C31', // interpolated
		'--grey-800': '#222226', // interpolated
		'--grey-900': '#18181B', // text
	},
	dark: {
		'--grey-50': '#18181B',
		'--grey-100': '#232326',
		'--grey-200': '#2A2A2E',
		'--grey-300': '#3F3F46',
		'--grey-400': '#A1A1AA',
		'--grey-500': '#BBBBC1',
		'--grey-600': '#D4D4D8',
		'--grey-700': '#E4E4E7',
		'--grey-800': '#ECECEE',
		'--grey-900': '#F4F4F5',
	},
}

// [foreground, background, minimum ratio, what it is]
const pairs: [string, string, number, string][] = [
	...(['--yy-bg-canvas', '--yy-bg-subtle', '--yy-bg-muted', '--yy-bg-emphasis'] as const).flatMap(bg => [
		['--yy-text', bg, 4.5, 'headings, primary text'] as [string, string, number, string],
		['--yy-text-secondary', bg, 4.5, 'body copy'] as [string, string, number, string],
		['--yy-text-muted', bg, 4.5, 'meta, captions, disabled button label'] as [string, string, number, string],
	]),
	['--yy-on-primary', '--yy-primary', 4.5, 'primary button'],
	['--yy-on-inverse', '--yy-inverse-surface', 4.5, 'toast'],
	['--switch-view-color', '--switch-view-active-background', 4.5, 'active view in the view switcher'],
	['--text', '--site-background', 4.5, 'Vikunja --text on the page'],
	['--text-strong', '--scheme-main', 4.5, 'Vikunja --text-strong on cards'],
	['--text-muted', '--scheme-main', 4.5, 'Vikunja --text-muted on cards'],
	['--link', '--site-background', 4.5, 'links'],
	...(['--grey-400', '--grey-500', '--grey-600', '--grey-700', '--grey-800', '--grey-900'] as const).flatMap(grey => [
		[grey, '--scheme-main', 4.5, `Vikunja text that uses ${grey}`] as [string, string, number, string],
		[grey, '--grey-100', 4.5, `${grey} text on the sidebar hover fill`] as [string, string, number, string],
	]),
	['--input-placeholder-color', '--input-background-color', 4.5, 'input placeholder'],
	['--yy-success-fg', '--yy-success-bg', 4.5, 'success chip'],
	['--yy-warning-fg', '--yy-warning-bg', 4.5, 'warning chip'],
	['--yy-danger-fg', '--yy-danger-bg', 4.5, 'danger chip, danger menu item on hover'],
	['--yy-danger-fg', '--yy-bg-canvas', 4.5, 'danger button text'],
	['--danger-text', '--scheme-main', 4.5, 'Vikunja --danger-text'],
	['#FFFFFF', '--danger', 4.5, 'white text on the danger fill'],
	['#FFFFFF', '--success', 4.5, 'white text on the success fill'],
	['--warning-invert', '--warning', 4.5, 'text on the warning fill'],
	['--yy-control-border', '--yy-bg-canvas', 3, 'input and secondary button border'],
	['--yy-control-border', '--yy-bg-subtle', 3, 'input border on a subtle surface'],
	['--yy-danger-fg', '--yy-bg-canvas', 3, 'danger button border'],
	['--yy-primary', '--yy-bg-canvas', 3, 'focus ring on the canvas'],
	['--yy-primary', '--yy-bg-subtle', 3, 'focus ring in the sidebar'],
	['--yy-text-muted', '--yy-bg-canvas', 3, 'header icon buttons'],
]

describe('Yuyu theme layer', () => {
	for (const theme of themes) {
		describe(theme, () => {
			const vars = resolveTheme(yuyuScss, theme)

			it('rebuilds the grey scale from the Yuyu neutrals', () => {
				const actual = Object.fromEntries(Object.keys(expectedGreys[theme]).map(name => [name, color(vars, name)]))
				expect(actual).toEqual(expectedGreys[theme])
			})

			it('maps primary onto the ink', () => {
				expect(color(vars, '--primary')).toBe(theme === 'light' ? '#18181B' : '#F4F4F5')
				expect(color(vars, '--danger')).toBe('#B91C1C')
				expect(color(vars, '--success')).toBe('#15803D')
				expect(color(vars, '--warning')).toBe('#F59E0B')
			})

			it('meets contrast for every text/background pair', () => {
				const rows = pairs.map(([fg, bg, min, what]) => {
					const ratio = contrast(color(vars, fg), color(vars, bg))
					return {pair: `${fg} on ${bg}`, what, ratio: Math.round(ratio * 100) / 100, min, ok: ratio >= min}
				})
				if (process.env.YUYU_CONTRAST_REPORT) console.table(rows)
				expect(rows.filter(row => !row.ok)).toEqual([])
			})
		})
	}

	it('fails a pair that is too light (the check itself works)', () => {
		expect(contrast('#D4D3CE', '#FFFFFF')).toBeLessThan(3)
		expect(contrast('#18181B', '#FFFFFF')).toBeGreaterThan(17)
	})
})
