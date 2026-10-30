import {describe, expect, it} from 'vitest'

import {assigneeAction} from './assigneeAccess'
import {PERMISSIONS} from '@/constants/permissions'

describe('assigneeAction', () => {
	it('assigns people who can already see the project, whatever your role', () => {
		for (const permission of [PERMISSIONS.READ, PERMISSIONS.READ_WRITE, PERMISSIONS.ADMIN, undefined]) {
			expect(assigneeAction(true, permission)).toBe('assign')
		}
	})

	it('offers to share only to project admins', () => {
		expect(assigneeAction(false, PERMISSIONS.ADMIN)).toBe('share-assign')
	})

	it('never shares for read & write or read members', () => {
		expect(assigneeAction(false, PERMISSIONS.READ_WRITE)).toBe('admin-needed')
		expect(assigneeAction(false, PERMISSIONS.READ)).toBe('admin-needed')
		expect(assigneeAction(false, undefined)).toBe('admin-needed')
	})
})
