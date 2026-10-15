import test from 'node:test';
import assert from 'node:assert/strict';
import { parseEditorJSON, propertyPatch } from './editor.js';

test('unsafe IDs and integer attributes cannot be rounded into writes', () => {
  assert.throws(() => parseEditorJSON('{"id":18446744073709551615}'), /safely represent/);
  assert.throws(() => parseEditorJSON('{"properties":{"n":9007199254740993}}'), /safely represent/);
  assert.equal(parseEditorJSON('{"id":0}').id, 0);
  assert.throws(() => parseEditorJSON('{"n":1.234567890123456789}'), /safely represent/);
  assert.equal(parseEditorJSON('{"n":1.25,"s":"1.234567890123456789"}').n, 1.25);
});

test('patch preserves empty string, explicit null, absence, and pointer names', () => {
  assert.deepEqual(propertyPatch({ name: 'old', nullable: 'x', remove: 1, 'a/b~c': 1 },
    { name: '', nullable: null, 'a/b~c': 2, added: false }), [
    { op: 'replace', path: '/properties/name', value: '' },
    { op: 'replace', path: '/properties/nullable', value: null },
    { op: 'remove', path: '/properties/remove' },
    { op: 'replace', path: '/properties/a~1b~0c', value: 2 },
    { op: 'add', path: '/properties/added', value: false },
  ]);
  assert.deepEqual(propertyPatch({ n: 1 }, { n: 1 }), []);
});
