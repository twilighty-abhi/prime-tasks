// True while a text field outside `editorDom` has focus, e.g. the prompt for a link or image alt text.
export function otherInputHasFocus(editorDom: Element): boolean {
	const active = document.activeElement
	return active instanceof Element
		&& !editorDom.contains(active)
		&& active.matches('input, textarea, select')
}
