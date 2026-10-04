// An item checked off stays where it was until the list is opened again, even though the server
// no longer returns it in a list of open items.

export function keepInPlace<T extends { id: string }>(previous: readonly T[], next: readonly T[], keep: (id: string) => T | undefined): T[] {
  const present = new Set(next.map(x => x.id));
  const out = [...next];
  previous.forEach((old, index) => {
    if (present.has(old.id)) return;
    const kept = keep(old.id);
    if (!kept) return;
    // Put it back after the nearest earlier neighbour that is still listed.
    let at = 0;
    for (let k = index - 1; k >= 0; k--) {
      const pos = out.findIndex(x => x.id === previous[k]!.id);
      if (pos >= 0) { at = pos + 1; break; }
    }
    out.splice(at, 0, kept);
    present.add(old.id);
  });
  return out;
}
