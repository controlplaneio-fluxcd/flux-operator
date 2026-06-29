// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// termToTest compiles one lowercased term into a "contains" predicate over a
// lowercased haystack. A `*` acts as a wildcard: the term is split on `*` and its
// non-empty segments must appear in order anywhere in the haystack (unanchored, to
// keep the contains semantics). Scanning with indexOf is linear and
// backtracking-free, unlike a chained-`.*` regex on crafted input.
function termToTest(term) {
  if (!term.includes('*')) {
    return (h) => h.includes(term)
  }
  const segs = term.split('*').filter(Boolean)
  // A term of only `*` (e.g. `*` or `**`) selects everything.
  if (segs.length === 0) return () => true
  return (h) => {
    let pos = 0
    for (const seg of segs) {
      const idx = h.indexOf(seg, pos)
      if (idx === -1) return false
      pos = idx + seg.length
    }
    return true
  }
}

/**
 * compileSearch - compiles a free-text inventory query into a predicate over
 * inventory items. The query is split on whitespace into terms matched against each
 * item's combined, lowercased haystack (`name namespace kind apiVersion`):
 *   - a plain term is an include — the haystack must contain it;
 *   - a `!`-prefixed term is an exclude — no item whose haystack contains it passes;
 *   - `*` is a wildcard within a term (e.g. `kube-*-config`).
 * An item passes when it matches every include and no exclude. An empty query
 * matches everything. Matching is case-insensitive.
 *
 * @param {string} query - The free-text search query
 * @returns {(item: {name?: string, namespace?: string, kind?: string, apiVersion?: string}) => boolean}
 */
export function compileSearch(query) {
  const tokens = (query || '').toLowerCase().split(/\s+/).filter(Boolean)
  const includes = []
  const excludes = []
  for (const t of tokens) {
    if (t[0] === '!') { if (t.length > 1) excludes.push(t.slice(1)) }
    else includes.push(t)
  }
  if (includes.length === 0 && excludes.length === 0) return () => true
  const inc = includes.map(termToTest)
  const exc = excludes.map(termToTest)
  return (item) => {
    const h = `${item.name || ''} ${item.namespace || ''} ${item.kind || ''} ${item.apiVersion || ''}`.toLowerCase()
    if (exc.some(t => t(h))) return false
    return inc.every(t => t(h))
  }
}
