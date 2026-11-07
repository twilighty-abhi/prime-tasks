import {describe, it, expect, afterEach} from 'vitest'

import {otherInputHasFocus} from './otherInputHasFocus'

describe('otherInputHasFocus', () => {
	afterEach(() => {
		document.body.innerHTML = ''
	})

	function setup() {
		const editorDom = document.createElement('div')
		editorDom.contentEditable = 'true'
		document.body.append(editorDom)
		return editorDom
	}

	it('is false when nothing has focus', () => {
		expect(otherInputHasFocus(setup())).toBe(false)
	})

	it('is false when the editor itself has focus', () => {
		const editorDom = setup()
		editorDom.focus()
		expect(otherInputHasFocus(editorDom)).toBe(false)
	})

	it('is false when a button has focus', () => {
		const editorDom = setup()
		const button = document.createElement('button')
		document.body.append(button)
		button.focus()
		expect(otherInputHasFocus(editorDom)).toBe(false)
	})

	it.each(['input', 'textarea', 'select'])('is true when a %s outside the editor has focus', (tag) => {
		const editorDom = setup()
		const field = document.createElement(tag)
		document.body.append(field)
		field.focus()
		expect(otherInputHasFocus(editorDom)).toBe(true)
	})

	it('is false for an input inside the editor', () => {
		const editorDom = setup()
		const field = document.createElement('input')
		editorDom.append(field)
		field.focus()
		expect(otherInputHasFocus(editorDom)).toBe(false)
	})
})
