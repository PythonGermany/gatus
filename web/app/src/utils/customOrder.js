// Helpers for the user-defined (drag and drop) order of dashboard items.
// Orders are arrays of item keys stored in localStorage; items missing from an order are appended at the end.

export const CUSTOM_ORDER_STORAGE_KEYS = {
  endpoints: 'gatus:custom-order:endpoints',
  suites: 'gatus:custom-order:suites',
  groups: 'gatus:custom-order:groups'
}

export const loadOrder = (storageKey) => {
  try {
    const saved = localStorage.getItem(storageKey)
    if (!saved) return []
    const parsed = JSON.parse(saved)
    if (Array.isArray(parsed) && parsed.every(key => typeof key === 'string')) {
      return parsed
    }
    throw new Error('invalid custom order')
  } catch (e) {
    console.warn(`Failed to parse saved custom order ${storageKey}:`, e)
    try {
      localStorage.removeItem(storageKey)
    } catch (_) {
      // ignore unavailable storage
    }
    return []
  }
}

export const saveOrder = (storageKey, order) => {
  try {
    if (!order || order.length === 0) {
      localStorage.removeItem(storageKey)
    } else {
      localStorage.setItem(storageKey, JSON.stringify(order))
    }
  } catch (e) {
    console.warn(`Failed to save custom order ${storageKey}:`, e)
  }
}

// Sorts items by their key's position in order. Items without a position keep their incoming relative order
// and are placed after all ordered items.
export const applyOrder = (items, order, keyFn = item => item) => {
  if (!order || order.length === 0) return items
  const positions = new Map(order.map((key, index) => [key, index]))
  return items
    .map((item, index) => ({ item, index, position: positions.get(keyFn(item)) }))
    .sort((a, b) => {
      const aOrdered = a.position !== undefined
      const bOrdered = b.position !== undefined
      if (aOrdered && bOrdered) return a.position - b.position
      if (aOrdered) return -1
      if (bOrdered) return 1
      return a.index - b.index
    })
    .map(entry => entry.item)
}

// Returns a new full order (covering exactly allKeys) in which draggedKey takes targetKey's slot.
export const moveKey = (order, allKeys, draggedKey, targetKey) => {
  const result = applyOrder([...new Set(allKeys)], order)
  const from = result.indexOf(draggedKey)
  const to = result.indexOf(targetKey)
  if (from === -1 || to === -1 || from === to) return result
  result.splice(from, 1)
  result.splice(to, 0, draggedKey)
  return result
}
