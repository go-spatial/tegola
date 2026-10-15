// Reject integers before they can be submitted with JavaScript rounding.
// Larger IDs and exact decimal editing require a lossless external client.
export function parseEditorJSON(text) {
	// Inspect numeric tokens before JSON.parse; a reviver alone sees already
	// rounded decimal fractions. Quoted strings are consumed as whole tokens.
	for (const match of text.matchAll(/"(?:\\.|[^"\\])*"|(-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)/g)) {
		if (!match[1]) continue;
		const token = match[1];
		const number = Number(token);
		const digits = token.split(/[eE]/)[0].replace(/[-.]/g, '').replace(/^0+|0+$/g, '');
		if ((/[.eE]/.test(token) && digits.length > 15) ||
			(number === 0 && /[1-9]/.test(token.split(/[eE]/)[0]))) {
			throw new Error('This editor cannot safely represent this decimal. Use a lossless external client.');
		}
	}
  return JSON.parse(text, (_key, value) => {
    if (typeof value === 'number' && (!Number.isFinite(value) ||
        (Number.isInteger(value) && !Number.isSafeInteger(value)))) {
      throw new Error('This editor cannot safely represent this number. Use a lossless external client.');
    }
    return value;
  });
}

export function propertyPatch(original, edited) {
  const patch = [];
  for (const name of new Set([...Object.keys(original), ...Object.keys(edited)])) {
    const path = '/properties/' + name.replace(/~/g, '~0').replace(/\//g, '~1');
    const before = Object.hasOwn(original, name);
    const after = Object.hasOwn(edited, name);
    if (!after) patch.push({ op: 'remove', path });
    else if (!before) patch.push({ op: 'add', path, value: edited[name] });
    else if (JSON.stringify(original[name]) !== JSON.stringify(edited[name])) {
      patch.push({ op: 'replace', path, value: edited[name] });
    }
  }
  return patch;
}
