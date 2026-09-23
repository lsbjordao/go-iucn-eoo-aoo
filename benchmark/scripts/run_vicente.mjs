#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const [repoArg, datasetsArg, outputArg, revisionArg = 'unknown'] = process.argv.slice(2);
if (!repoArg || !datasetsArg || !outputArg) {
  console.error('usage: node run_vicente.mjs <repo> <datasets> <output.csv> [revision]');
  process.exit(2);
}

const repo = path.resolve(repoArg);
const datasetsDir = path.resolve(datasetsArg);
const output = path.resolve(outputArg);
const pkg = JSON.parse(fs.readFileSync(path.join(repo, 'package.json'), 'utf8'));
const calculator = require(path.join(repo, 'dist', 'index.js'));
const { AOO, EOO } = calculator;
if (!AOO || !EOO) throw new Error('reference package does not export AOO and EOO');

const fields = [
  'dataset', 'tool', 'tool_version', 'tool_revision', 'mode',
  'eoo_raw_km2', 'eoo_assessment_km2', 'aoo_km2', 'occupied_cells',
  'input_records', 'unique_coordinates', 'error', 'settings', 'notes',
];

function csvCell(value) {
  if (value === null || value === undefined) return '';
  const s = String(value);
  return /[",\n\r]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s;
}

function uniqueCoordinates(points) {
  return new Set(points.map(p => `${Number(p.lon).toPrecision(15)},${Number(p.lat).toPrecision(15)}`)).size;
}

function message(err) {
  return err instanceof Error ? err.message : String(err);
}

const rawDir = path.join(path.dirname(output), 'raw', 'vicentecalfo');
fs.mkdirSync(rawDir, { recursive: true });
const rows = [];
for (const file of fs.readdirSync(datasetsDir).filter(x => x.endsWith('.json')).sort()) {
  const dataset = path.basename(file, '.json');
  const points = JSON.parse(fs.readFileSync(path.join(datasetsDir, file), 'utf8'));
  const coordinates = points.map(p => ({ longitude: Number(p.lon), latitude: Number(p.lat) }));
  const row = {
    dataset,
    tool: 'vicentecalfo/eoo-aoo-calculator',
    tool_version: pkg.version || '',
    tool_revision: revisionArg,
    mode: 'package-default',
    eoo_assessment_km2: '',
    input_records: points.length,
    unique_coordinates: uniqueCoordinates(points),
    settings: JSON.stringify({ gridWidthInKm: 2 }),
    notes: 'AOO and EOO invoked independently through the package library API on identical coordinates',
  };
  const errors = [];
  let aoo = null;
  let eoo = null;
  try {
    aoo = new AOO({ coordinates }).calculate({ gridWidthInKm: 2 });
    row.aoo_km2 = aoo?.areaInSquareKm ?? '';
    row.occupied_cells = aoo?.totalOccupiedGrids ?? '';
  } catch (err) {
    errors.push(`AOO: ${message(err)}`);
  }
  try {
    eoo = new EOO({ coordinates }).calculate();
    row.eoo_raw_km2 = eoo?.areaInSquareKm ?? '';
  } catch (err) {
    errors.push(`EOO: ${message(err)}`);
  }
  row.error = errors.join(' | ');
  fs.writeFileSync(
    path.join(rawDir, `${dataset}.json`),
    JSON.stringify({ package: pkg.name, version: pkg.version, revision: revisionArg, aoo, eoo, errors }, null, 2) + '\n',
  );
  rows.push(row);
}

fs.mkdirSync(path.dirname(output), { recursive: true });
const lines = [fields.join(',')];
for (const row of rows) lines.push(fields.map(f => csvCell(row[f] ?? '')).join(','));
fs.writeFileSync(output, lines.join('\n') + '\n');
console.log(`wrote ${rows.length} rows to ${output}`);
