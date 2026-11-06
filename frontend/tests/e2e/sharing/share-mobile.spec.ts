import type {Locator} from '@playwright/test'
import {test, expect} from '../../support/fixtures'
import {TaskFactory} from '../../factories/task'
import {createProjects} from '../project/prepareProjects'

// prime-tasks F5: the Share button and dialog at phone width, and the task breadcrumb's contrast.

async function box(locator: Locator) {
	const b = await locator.boundingBox()
	expect(b).not.toBeNull()
	return b!
}

function overlaps(a: {x: number, y: number, width: number, height: number}, b: {x: number, y: number, width: number, height: number}) {
	return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height
}

test.describe('at 360px', () => {
	test.use({viewport: {width: 360, height: 780}})

	test('the Share button is icon-only and clear of the search button', async ({authenticatedPage: page}) => {
		await createProjects(1)
		await page.goto('/projects/1/1')
		const share = page.locator('.project-share-button')
		await expect(share).toBeVisible()
		await expect(share).toHaveAttribute('aria-label', 'Share')
		await expect(share.locator('.project-share-button__label')).toBeHidden()
		const search = page.locator('.navbar-end').getByRole('button').first()
		expect(overlaps(await box(share), await box(search))).toBe(false)
	})

	test('the share dialog has no gap above the search, a contained role select and a round close button', async ({authenticatedPage: page}) => {
		await createProjects(1)
		await page.goto('/projects/1/settings/share')
		const dialog = page.locator('.share-dialog')
		await expect(dialog.locator('.share-dialog__row').first()).toBeVisible()

		const header = await box(page.locator('.card-header').filter({has: page.locator('.card-header-title')}).first())
		const search = await box(dialog.getByRole('combobox', {name: 'Search for people or teams'}))
		expect(search.y - (header.y + header.height)).toBeLessThan(40)

		const wrapper = await box(dialog.locator('.share-dialog__add .select'))
		const select = await box(dialog.locator('.share-dialog__add .select select'))
		expect(Math.abs(wrapper.width - select.width)).toBeLessThan(2)

		const close = page.locator('.card-header-icon.close')
		await close.focus()
		const ring = await close.evaluate(el => {
			const cs = getComputedStyle(el)
			return {radius: parseFloat(cs.borderTopLeftRadius), width: el.getBoundingClientRect().width, outline: cs.outlineStyle}
		})
		expect(ring.outline).toBe('solid')
		expect(ring.radius).toBeGreaterThanOrEqual(ring.width / 2)
	})
})

for (const colorScheme of ['light', 'dark'] as const) {
	test(`the task breadcrumb separator meets 3:1 contrast (${colorScheme})`, async ({authenticatedPage: page}) => {
		const projects = await createProjects(1)
		const [task] = await TaskFactory.create(1, {project_id: projects[0].id})
		await page.emulateMedia({colorScheme})
		await page.goto(`/tasks/${task.id}`)
		const separator = page.locator('.task-breadcrumb-separator').first()
		await expect(separator).toBeVisible()
		const ratio = await separator.evaluate(el => {
			const rgb = (c: string) => (c.match(/[\d.]+/g) ?? []).slice(0, 3).map(Number)
			const lum = (c: number[]) => {
				const [r, g, b] = c.map(v => {
					v /= 255
					return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
				})
				return 0.2126 * r + 0.7152 * g + 0.0722 * b
			}
			let bg = 'rgba(0, 0, 0, 0)'
			for (let e: Element | null = el; e && /rgba\(0, 0, 0, 0\)|transparent/.test(bg); e = e.parentElement) {
				bg = getComputedStyle(e).backgroundColor
			}
			const [a, b] = [lum(rgb(getComputedStyle(el).color)), lum(rgb(bg))].sort((x, y) => y - x)
			return (a + 0.05) / (b + 0.05)
		})
		expect(ratio).toBeGreaterThanOrEqual(3)
	})
}
