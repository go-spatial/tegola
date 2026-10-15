<template>
  <section class="editor-panel">
    <h3>Feature properties editor</h3>
    <p>Edits attributes from the source feature. Existing geometry is preserved.
      Geometry drawing and exact large-number editing require an external client.</p>
    <fieldset :disabled="busy || uncertain">
      <label>Feature API path <input v-model="apiPath" placeholder="features" /></label>
      <button @click="loadCollections">Connect</button>
      <label v-if="collections.length">Collection
        <select v-model="selectedCollection" @change="loadSchema">
          <option v-for="c in collections" :key="c.id" :value="c.id">{{ c.title || c.id }}</option>
        </select>
      </label>
      <template v-if="schema">
        <label>Feature ID <input v-model="requestedId" inputmode="numeric" /></label>
        <button @click="loadFeature">Load source feature</button>
        <button @click="reset">New feature</button>
        <h4>{{ isEdit ? `Feature ${editingId}` : 'New feature' }}</h4>
        <SchemaForm :schema="schema" :model-value="formData" @update:model-value="changeForm" />
        <label v-if="!isEdit">Geometry (GeoJSON object or null)
          <textarea v-model="geometryText" rows="5" />
        </label>
        <div class="actions">
          <button @click="save" :disabled="isEdit && !etag">{{ isEdit ? 'Save properties' : 'Create' }}</button>
          <button v-if="isEdit" @click="remove" :disabled="!etag">Delete</button>
          <button @click="cancel">Cancel changes</button>
          <button @click="undo" :disabled="!undoStack.length">Undo</button>
          <button @click="redo" :disabled="!redoStack.length">Redo</button>
        </div>
      </template>
    </fieldset>
    <p v-if="message" role="status" :class="['message', messageType]">{{ message }}</p>
    <details v-if="schema">
      <summary>Copy current attribute draft</summary>
      <textarea :value="JSON.stringify(formData, null, 2)" readonly rows="8" aria-label="Current attribute draft" />
    </details>
    <p v-if="uncertain">{{ confirmed ? 'The write committed, but its representation could not be loaded.' : 'Do not retry this write blindly.' }} Verify the source with an external client,
      then reconnect this editor. Your draft remains visible above.</p>
  </section>
</template>

<script>
import axios from 'axios';
import SchemaForm from './SchemaForm.vue';
import { store } from '../globals/store';
import { map } from '../globals/map';
import { parseEditorJSON, propertyPatch } from '../globals/editor';

const clone = value => JSON.parse(JSON.stringify(value));
// Fetch raw JSON so Axios cannot round a feature ID before admission.
const jsonResponse = { transformResponse: [text => text ? parseEditorJSON(text) : null] };
export default {
  name: 'EditorPanel',
  components: { SchemaForm },
  data() {
    return { apiPath: 'features', connectedRoot: '', collections: [], selectedCollection: '',
      schema: null, formData: {}, original: {}, requestedId: '', editingId: null,
      geometryText: 'null', etag: null, busy: false, uncertain: false, confirmed: false,
      message: '', messageType: 'info', undoStack: [], redoStack: [] };
  },
  computed: { isEdit() { return this.editingId !== null; } },
  created() { this.loadCollections(); },
  methods: {
    collectionURL() {
      return `${this.connectedRoot}/collections/${encodeURIComponent(this.selectedCollection)}`;
    },
    showMessage(message, type = 'info') { this.message = message; this.messageType = type; },
    refreshMap() {
      // Recreate sources to discard already loaded vector tiles after a write.
      // Tile requests in a write deployment use no-store on the server.
      try { if (map) map.setStyle(map.getStyle(), { diff: false }); }
      catch (_) { this.message += ' Map refresh failed; reload the viewer.'; }
    },
    async loadCollections() {
      this.busy = true;
      this.schema = null;
      this.collections = [];
      this.reset();
      try {
        const root = new URL((store.apiRoot || '') + this.apiPath.replace(/\/$/, ''), window.location.href);
        if (root.origin !== window.location.origin || root.search || root.hash) {
          throw new Error('Use a same-origin Feature API path without query or fragment.');
        }
        this.connectedRoot = root.href.replace(/\/$/, '');
        const r = await axios.get(`${this.connectedRoot}/collections`, jsonResponse);
        this.collections = r.data.collections || [];
        this.selectedCollection = this.collections[0]?.id || '';
        if (this.selectedCollection) await this.loadSchema();
        else this.showMessage('No published feature collections.');
      } catch (e) { this.showMessage('Connection failed: ' + e.message, 'error'); }
      finally { this.busy = false; }
    },
    async loadSchema() {
      this.busy = true;
      this.schema = null;
      this.reset();
      try {
        const r = await axios.get(`${this.collectionURL()}/schema`, jsonResponse);
        this.schema = r.data;
      } catch (e) { this.showMessage('Write schema unavailable for this collection: ' + e.message, 'error'); }
      finally { this.busy = false; }
    },
    writableProperties(properties) {
      const allowed = this.schema.properties?.properties?.properties || {};
      return Object.fromEntries(Object.entries(properties || {}).filter(([name]) =>
        Object.hasOwn(allowed, name) && !allowed[name].readOnly));
    },
    async loadFeature() {
      if (!/^(0|[1-9][0-9]*)$/.test(this.requestedId)) {
        this.showMessage('Enter a decimal feature ID.', 'error'); return;
      }
      this.busy = true;
      this.etag = null;
      try {
        const r = await axios.get(`${this.collectionURL()}/items/${encodeURIComponent(this.requestedId)}`, jsonResponse);
        if (String(r.data.id) !== this.requestedId || !r.headers.etag || r.headers.etag.startsWith('W/')) {
          throw new Error('Source identity or strong ETag is unavailable; editing is disabled.');
        }
        // Properties and validator must come from the same source response.
        this.editingId = this.requestedId;
        this.original = this.writableProperties(r.data.properties);
        this.formData = clone(this.original);
        this.etag = r.headers.etag;
        this.undoStack = []; this.redoStack = [];
        this.showMessage('Loaded source feature.');
      } catch (e) { this.showMessage('Load failed: ' + e.message, 'error'); }
      finally { this.busy = false; }
    },
    reset() {
      this.editingId = null; this.formData = {}; this.original = {}; this.etag = null;
      this.undoStack = []; this.redoStack = []; this.geometryText = 'null'; this.message = '';
    },
    changeForm(value) {
      this.undoStack.push(clone(this.formData)); this.redoStack = []; this.formData = value;
    },
    cancel() { this.changeForm(clone(this.original)); this.geometryText = 'null'; },
    undo() { this.redoStack.push(clone(this.formData)); this.formData = this.undoStack.pop(); },
    redo() { this.undoStack.push(clone(this.formData)); this.formData = this.redoStack.pop(); },
    writeError(e) {
      if (e.response?.status === 412) {
        this.etag = null;
        this.showMessage('Conflict: your draft is preserved. Copy it before reloading the source; changes were not saved.', 'error');
      } else if (!e.response || e.response.status >= 500) {
        this.uncertain = true;
        this.showMessage('Write outcome is unknown. The draft is preserved; verify the source before another write.', 'error');
      } else {
        this.showMessage('Write rejected: ' + (e.response.data?.description || e.message), 'error');
      }
    },
    async save() {
      for (const input of this.$el.querySelectorAll('input')) {
        if (!input.reportValidity()) return;
      }
      this.busy = true;
      let sent = false;
      try {
        // This also rejects nonfinite/unsafe numeric values entered into forms.
        const props = parseEditorJSON(JSON.stringify(this.formData));
        let r;
        if (this.isEdit) {
          if (!this.etag) throw new Error('Reload the source before saving.');
          const patch = propertyPatch(this.original, props);
          if (!patch.length) { this.showMessage('No attribute changes.'); return; }
          sent = true;
          r = await axios.patch(`${this.collectionURL()}/items/${encodeURIComponent(this.editingId)}`, patch,
            { ...jsonResponse, headers: { 'If-Match': this.etag, 'Content-Type': 'application/json-patch+json' } });
        } else {
          const geometry = parseEditorJSON(this.geometryText);
          if (geometry !== null && (typeof geometry !== 'object' || Array.isArray(geometry))) {
            throw new Error('Geometry must be a GeoJSON geometry object or null.');
          }
          sent = true;
          r = await axios.post(`${this.collectionURL()}/items`, { type: 'Feature', properties: props, geometry },
            { ...jsonResponse, headers: { 'Content-Type': 'application/geo+json' } });
        }
        if (!r.data || r.data.type !== 'Feature') {
          this.confirmed = r.headers['tegola-commit-status'] === 'committed';
          this.uncertain = true;
          this.etag = null;
          this.showMessage(this.confirmed ? 'Write committed; source readback is unavailable. Do not repeat the write.' :
            'Write returned without a source representation; verify the source before retrying.', 'info');
          if (this.confirmed) this.refreshMap();
          return;
        }
        this.editingId = String(r.data.id); this.requestedId = this.editingId;
        this.original = this.writableProperties(r.data.properties); this.formData = clone(this.original);
        this.etag = r.headers.etag || null; this.undoStack = []; this.redoStack = [];
        this.showMessage('Saved and confirmed by the persisted source representation.', 'success');
        this.refreshMap();
      } catch (e) {
        if (sent) this.writeError(e);
        else this.showMessage(e.message, 'error');
      } finally { this.busy = false; }
    },
    async remove() {
      if (!this.etag || !confirm(`Delete feature ${this.editingId}?`)) return;
      this.busy = true;
      try {
        await axios.delete(`${this.collectionURL()}/items/${encodeURIComponent(this.editingId)}`,
          { headers: { 'If-Match': this.etag } });
        this.reset(); this.showMessage('Deleted.', 'success'); this.refreshMap();
      } catch (e) { this.writeError(e); }
      finally { this.busy = false; }
    },
  },
};
</script>

<style scoped>
.editor-panel { position: absolute; top: 55px; right: 0; bottom: 0; overflow: auto; padding: 16px; width: min(460px, 90vw); background: #20242b; z-index: 2; }
.editor-panel label { display: block; margin: 8px 0; }
.editor-panel input, .editor-panel textarea { display: block; width: 95%; }
.actions { margin: 12px 0; }
button { margin: 4px; }
.message { padding: 8px; border: 1px solid currentColor; }
.message.success { color: #b3efc4; }
.message.error { color: #ffc6cb; }
</style>
