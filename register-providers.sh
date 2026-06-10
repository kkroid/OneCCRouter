#!/bin/sh
set -e

echo "=== Registering Providers ==="

bun -e "
import { readFileSync, writeFileSync } from 'fs';

const API = 'http://9router:3456/api';
const providers = JSON.parse(readFileSync('/providers.json', 'utf-8'));
let cookie = '';

async function api(method, url, body) {
  const headers = { 'Content-Type': 'application/json' };
  if (cookie) headers['Cookie'] = cookie;
  const opts = { method, headers };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(url, opts);
  const sc = res.headers.get('set-cookie');
  if (sc) cookie = sc.split(';')[0];
  const text = await res.text().catch(() => '');
  try { return { ok: res.ok, data: JSON.parse(text) }; }
  catch { return { ok: res.ok, data: text, status: res.status }; }
}
const post = (url, body) => api('POST', url, body);
const get = (url) => api('GET', url);
const del = (url) => api('DELETE', url);

// Login
await post(API + '/auth/login', { password: '123456' });

// Delete old nodes
const existing = await get(API + '/provider-nodes');
const knownPrefixes = providers.map(p => p.prefix);
if (existing?.nodes) {
  for (const n of existing.nodes) {
    if (knownPrefixes.includes(n.prefix)) {
      await del(API + '/provider-nodes/' + n.id);
      console.log('Deleted old node:', n.prefix);
    }
  }
}

// Register each provider
const models = [];
for (const p of providers) {
  const r = await post(API + '/provider-nodes', {
    name: p.name,
    prefix: p.prefix,
    baseUrl: p.baseUrl,
    type: 'anthropic-compatible'
  });
  console.log(p.prefix + ' node:', JSON.stringify(r.data));

  const nodeId = r.data?.node?.id;
  if (nodeId) {
    const apiKey = p.apiKey || (p.apiKeyEnv ? (process.env[p.apiKeyEnv] || '') : '');
    const c = await post(API + '/providers', {
      provider: nodeId,
      name: p.name,
      apiKey,
      label: p.name
    });
    console.log(p.prefix + ' conn:', c.ok ? 'OK' : 'FAIL');
  }

  for (const m of (p.models || [])) {
    models.push({ id: p.prefix + '/' + m, name: p.name + ' ' + m });
  }
}

// Generate Claude Code settings
const settings = {
  apiKey: 'x',
  baseUrl: 'http://localhost:3456/v1',
  model: models[0]?.id || '',
  _availableModels: models
};
writeFileSync('/out/claude-code-settings.json', JSON.stringify(settings, null, 2) + '\n');
console.log('\nGenerated claude-code-settings.json with', models.length, 'models');

console.log('=== Done ===');
"
