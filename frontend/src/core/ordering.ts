/**
 * 拖拽排序逻辑层（P-09 分类拖拽，§14.5 #2 Must：原生 HTML5 draggable，不引库）
 *
 * 说明：后端 `UpdateAccountSortOrder` 尚未暴露 REST 端点，本期先落逻辑层
 * 与会话内排序；持久化端点开放后把结果逐账户上送即可（见批 2 已知遗留）。
 */

/** 把 `from` 位置的项移动到 `to` 位置；返回新数组（不可变），越界 / 相同位置原样返回。 */
export function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
  if (
    from < 0 ||
    to < 0 ||
    from >= list.length ||
    to >= list.length ||
    from === to ||
    !Number.isSafeInteger(from) ||
    !Number.isSafeInteger(to)
  ) {
    return list.slice()
  }
  const next = list.slice()
  const moved = next.splice(from, 1)
  next.splice(to, 0, moved[0])
  return next
}

/** 按 sort_order 升序稳定排序（同序号保持原相对顺序）。 */
export function sortBySortOrder<T>(
  list: readonly T[],
  getOrder: (item: T) => number,
): T[] {
  return list
    .slice()
    .map((item, index) => ({ item, index, order: getOrder(item) }))
    .sort((a, b) => {
      if (a.order !== b.order) {
        return a.order - b.order
      }
      return a.index - b.index
    })
    .map((entry) => entry.item)
}
