<template>
  <div class="schema-form">
    <div v-for="prop in properties" :key="prop.name" class="field">
      <label :for="'f_' + prop.name">{{ prop.title || prop.name }} ({{ prop.valueType }})</label>
      <select :value="state(prop.name)" :aria-label="prop.name + ' value state'" @change="setState(prop, $event.target.value)">
        <option value="absent">Omitted</option>
        <option v-if="prop.nullable" value="null">NULL</option>
        <option value="value">Value</option>
      </select>
      <template v-if="state(prop.name) === 'value'">
        <input v-if="prop.valueType === 'boolean'" :id="'f_' + prop.name" type="checkbox"
          :checked="modelValue[prop.name] === true" @change="setValue(prop.name, $event.target.checked)" />
        <input v-else :id="'f_' + prop.name" type="text" :value="modelValue[prop.name]"
          @input="inputValue(prop, $event.target)" />
      </template>
    </div>
  </div>
</template>

<script>
import { parseEditorJSON } from '../globals/editor';
export default {
  name: 'SchemaForm',
  props: { schema: { type: Object, required: true }, modelValue: { type: Object, default: () => ({}) } },
  emits: ['update:modelValue'],
  computed: {
    properties() {
      const props = this.schema.properties?.properties?.properties || {};
      return Object.entries(props).filter(([, p]) => !p.readOnly).map(([name, p]) => ({
        ...p, name, nullable: Array.isArray(p.type) && p.type.includes('null'),
        valueType: Array.isArray(p.type) ? p.type.find(t => t !== 'null') : p.type,
      }));
    },
  },
  methods: {
    state(name) { return !Object.hasOwn(this.modelValue, name) ? 'absent' : this.modelValue[name] === null ? 'null' : 'value'; },
    setValue(name, value) { this.$emit('update:modelValue', { ...this.modelValue, [name]: value }); },
    setState(prop, state) {
      const next = { ...this.modelValue };
      if (state === 'absent') delete next[prop.name];
      else if (state === 'null') next[prop.name] = null;
      else next[prop.name] = prop.valueType === 'boolean' ? false : ['integer', 'number'].includes(prop.valueType) ? 0 : '';
      this.$emit('update:modelValue', next);
    },
    inputValue(prop, input) {
      let value = input.value;
      input.setCustomValidity('');
      if (['integer', 'number'].includes(prop.valueType)) {
        if (!/^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$/.test(value)) {
          input.setCustomValidity('Enter a finite JSON number, or select NULL/omitted.'); return;
        }
        try { value = parseEditorJSON(value); }
        catch (e) { input.setCustomValidity(e.message); return; }
        if (!Number.isFinite(value) || (prop.valueType === 'integer' && !Number.isSafeInteger(value)) ||
            (Number.isInteger(value) && !Number.isSafeInteger(value))) {
          input.setCustomValidity('This number cannot be edited safely in this browser form.'); return;
        }
      }
      this.setValue(prop.name, value);
    },
  },
};
</script>

<style scoped>
.field { margin: 10px 0; }
label { display: block; font-weight: bold; margin-bottom: 4px; }
input[type="text"] { display: block; width: 95%; padding: 6px; }
</style>
