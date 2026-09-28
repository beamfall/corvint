export function visibleText(value, limit = 1600) {
  if (typeof value !== "string") return ""
  const text = value.replace(/[\x00-\x1f\x7f-\x9f\u061c\u200b-\u200f\u2028-\u202e\u2060-\u2064\u2066-\u2069\ufeff]/gu,
    ch => `\\u${ch.charCodeAt(0).toString(16).padStart(4, "0")}`)
  return text.length > limit ? text.slice(0, limit) + "… [display shortened]" : text
}

