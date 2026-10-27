import {PRODUCT_NAME} from '@/constants/product'

export function setTitle(title : undefined | string) {
	document.title = (typeof title === 'undefined' || title === '')
		? PRODUCT_NAME
		: `${title} | ${PRODUCT_NAME}`
}
