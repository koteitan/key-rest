#!/usr/bin/env node
/**
 * System test: starts test-server and key-rest daemon, registers all credentials,
 * and tests all 26 services through the Node.js key-rest client.
 */

import { execSync, execFileSync, spawn } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { setTimeout as sleep } from 'node:timers/promises';
import { createFetch } from '../../clients/node/dist/index.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..', '..');

// --- Helpers ---

function findFreePort() {
  const code = 'import socket; s=socket.socket(); s.bind(("",0)); print(s.getsockname()[1]); s.close()';
  return parseInt(execSync(`python3 -c '${code}'`, { encoding: 'utf-8' }).trim());
}

function parseCredentials(output) {
  const creds = {};
  let inCreds = false;
  for (const line of output.split('\n')) {
    if (line.startsWith('=== Test Credentials')) { inCreds = true; continue; }
    if (line.startsWith('====')) break;
    if (inCreds) {
      const fields = line.trim().split(/\s+/);
      if (fields.length >= 3) {
        creds[`${fields[0]}/${fields[1]}`] = fields[fields.length - 1];
      }
    }
  }
  return creds;
}

// --- Main ---

const work = mkdtempSync(join(tmpdir(), 'key-rest-node-test-'));
let testServerProc = null;
let daemonStarted = false;
const env = { ...process.env };
const keyRest = join(work, 'key-rest');

let passed = 0, failed = 0, total = 0;

async function main() {
  try {
    // --- Build ---
    console.log('=== Building ===');
    execSync(`go build -o ${join(work, 'test-server')} ./test-server/`, { cwd: ROOT, stdio: 'pipe' });
    execSync(`go build -o ${keyRest} ./cmd/key-rest/`, { cwd: ROOT, stdio: 'pipe' });

    const port = findFreePort();
    const cert = join(work, 'cert.pem');
    const keyFile = join(work, 'key.pem');

    // --- Start test-server ---
    console.log(`=== Starting test-server on port ${port} ===`);
    testServerProc = spawn(
      join(work, 'test-server'),
      ['-port', String(port), '-cert', cert, '-key', keyFile, '-gen-cert'],
      { stdio: ['ignore', 'pipe', 'pipe'] },
    );

    // Wait for credentials (closing "========================" line after "Test Credentials")
    let output = '';
    let seenHeader = false;
    await new Promise((resolve, reject) => {
      const timeout = setTimeout(() => reject(new Error('test-server timeout')), 10000);
      testServerProc.stdout.on('data', (chunk) => {
        output += chunk.toString();
        if (output.includes('Test Credentials')) seenHeader = true;
        // The closing line is all '=' characters (24+), appearing after the header
        if (seenHeader) {
          const lines = output.split('\n');
          for (let i = lines.length - 1; i >= 0; i--) {
            const trimmed = lines[i].trim();
            if (trimmed.length >= 20 && /^=+$/.test(trimmed)) {
              clearTimeout(timeout);
              resolve();
              return;
            }
          }
        }
      });
      testServerProc.on('exit', (code) => { clearTimeout(timeout); reject(new Error(`test-server exited: ${code}`)); });
    });

    const creds = parseCredentials(output);
    console.log(`Parsed ${Object.keys(creds).length} credentials`);

    // --- Register keys ---
    console.log('=== Registering keys ===');
    env.KEY_REST_DIR = join(work, 'keyrest-data');
    env.KEY_REST_SOCKET = join(env.KEY_REST_DIR, 'key-rest.sock');
    mkdirSync(env.KEY_REST_DIR, { recursive: true });

    const passphrase = 'system-test-passphrase';
    const base = `https://localhost:${port}`;

    function addKey(uri, urlPrefix, ...extraArgs) {
      const input = `${passphrase}\n${creds[uri]}\n`;
      execFileSync(keyRest, ['add', ...extraArgs, `user1/${uri}`, urlPrefix], {
        input, env, stdio: 'pipe',
      });
    }

    // Bearer token services
    for (const [uri, path] of [
      ['openai/api-key', '/openai/'], ['mistral/api-key', '/mistral/'],
      ['groq/api-key', '/groq/'], ['xai/api-key', '/xai/'],
      ['perplexity/api-key', '/perplexity/'], ['deepseek/api-key', '/deepseek/'],
      ['openrouter/api-key', '/openrouter/'], ['github/token', '/github/'],
      ['matrix/access-token', '/matrix/'], ['slack/bot-token', '/slack/'],
      ['sentry/auth-token', '/sentry/'], ['line/channel-access-token', '/line/'],
      ['notion/api-key', '/notion/'], ['discord/bot-token', '/discord/'],
      ['linear/api-key', '/linear/'],
    ]) addKey(uri, base + path);

    // Custom header services
    for (const [uri, path] of [
      ['anthropic/api-key', '/anthropic/'], ['exa/api-key', '/exa/'],
      ['brave/api-key', '/brave/'], ['gitlab/token', '/gitlab/'],
      ['bing/api-key', '/bing/'],
    ]) addKey(uri, base + path);

    // Query parameter services
    for (const [uri, path] of [
      ['gemini/api-key', '/gemini/'], ['google-search/api-key', '/google-search/'],
      ['trello/api-key', '/trello/'], ['trello/token', '/trello/'],
    ]) addKey(uri, base + path, '--allow-url');

    // Body field services
    addKey('tavily/api-key', base + '/tavily/', '--allow-body');

    // Path embedding services
    addKey('telegram/bot-token', base + '/telegram/', '--allow-url');

    // Basic auth
    addKey('atlassian/email', base + '/atlassian/');
    addKey('atlassian/token', base + '/atlassian/');

    // Trust test-server cert (must be before daemon start)
    env.SSL_CERT_FILE = cert;

    // --- Start daemon ---
    console.log('=== Starting key-rest daemon ===');
    execFileSync(keyRest, ['start'], { input: `${passphrase}\n`, env, stdio: ['pipe', 'ignore', 'ignore'] });
    daemonStarted = true;
    await sleep(3000);

    // Verify daemon is running
    const status = execFileSync(keyRest, ['status'], { env, encoding: 'utf-8' });
    if (!status.includes('running')) {
      console.log('FATAL: daemon failed to start');
      process.exit(1);
    }

    const socketPath = env.KEY_REST_SOCKET;
    const fetch = createFetch({ socketPath });

    // --- Test runner ---
    async function runTest(name, method, url, headers, body) {
      total++;
      try {
        const resp = await fetch(url, { method, headers, body });
        if (resp.status === 200) {
          const text = await resp.text();
          JSON.parse(text); // verify valid JSON
          console.log(`  PASS  ${name}`);
          passed++;
        } else {
          console.log(`  FAIL  ${name} (status ${resp.status})`);
          failed++;
        }
      } catch (e) {
        console.log(`  FAIL  ${name}`);
        console.log(`        ${e.message}`);
        failed++;
      }
    }

    console.log('');
    console.log('=== Testing all 26 services ===');
    console.log('');

    // --- Bearer token services (13) ---
    await runTest('openai', 'POST', `${base}/openai/v1/chat/completions`,
      { 'Authorization': 'Bearer key-rest://user1/openai/api-key', 'Content-Type': 'application/json' },
      '{"model":"gpt-4o"}');
    await runTest('mistral', 'POST', `${base}/mistral/v1/chat/completions`,
      { 'Authorization': 'Bearer key-rest://user1/mistral/api-key', 'Content-Type': 'application/json' },
      '{"model":"mistral-large-latest"}');
    await runTest('groq', 'POST', `${base}/groq/openai/v1/chat/completions`,
      { 'Authorization': 'Bearer key-rest://user1/groq/api-key', 'Content-Type': 'application/json' },
      '{"model":"llama-3.3-70b-versatile"}');
    await runTest('xai', 'POST', `${base}/xai/v1/chat/completions`,
      { 'Authorization': 'Bearer key-rest://user1/xai/api-key', 'Content-Type': 'application/json' },
      '{"model":"grok-3"}');
    await runTest('perplexity', 'POST', `${base}/perplexity/chat/completions`,
      { 'Authorization': 'Bearer key-rest://user1/perplexity/api-key', 'Content-Type': 'application/json' },
      '{"model":"sonar"}');
    await runTest('deepseek', 'POST', `${base}/deepseek/chat/completions`,
      { 'Authorization': 'Bearer key-rest://user1/deepseek/api-key', 'Content-Type': 'application/json' },
      '{"model":"deepseek-chat"}');
    await runTest('openrouter', 'POST', `${base}/openrouter/api/v1/chat/completions`,
      { 'Authorization': 'Bearer key-rest://user1/openrouter/api-key', 'Content-Type': 'application/json' },
      '{"model":"anthropic/claude-sonnet-4-20250514"}');
    await runTest('github', 'GET', `${base}/github/user/repos`,
      { 'Authorization': 'Bearer key-rest://user1/github/token' });
    await runTest('matrix', 'POST', `${base}/matrix/_matrix/client/v3/rooms/ROOM/send/m.room.message`,
      { 'Authorization': 'Bearer key-rest://user1/matrix/access-token', 'Content-Type': 'application/json' },
      '{"msgtype":"m.text","body":"test"}');
    await runTest('slack', 'POST', `${base}/slack/api/chat.postMessage`,
      { 'Authorization': 'Bearer key-rest://user1/slack/bot-token', 'Content-Type': 'application/json' },
      '{"channel":"C01234567","text":"test"}');
    await runTest('sentry', 'GET', `${base}/sentry/api/0/projects/`,
      { 'Authorization': 'Bearer key-rest://user1/sentry/auth-token' });
    await runTest('line', 'POST', `${base}/line/v2/bot/message/push`,
      { 'Authorization': 'Bearer key-rest://user1/line/channel-access-token', 'Content-Type': 'application/json' },
      '{"to":"U123","messages":[{"type":"text","text":"test"}]}');
    await runTest('notion', 'POST', `${base}/notion/v1/databases/DB/query`,
      { 'Authorization': 'Bearer key-rest://user1/notion/api-key', 'Content-Type': 'application/json' },
      '{}');

    // --- Custom header services (5) ---
    await runTest('anthropic', 'POST', `${base}/anthropic/v1/messages`,
      { 'X-Api-Key': 'key-rest://user1/anthropic/api-key', 'Content-Type': 'application/json' },
      '{"model":"claude-sonnet-4-20250514","max_tokens":1}');
    await runTest('exa', 'POST', `${base}/exa/search`,
      { 'X-Api-Key': 'key-rest://user1/exa/api-key', 'Content-Type': 'application/json' },
      '{"query":"test","type":"neural"}');
    await runTest('brave', 'GET', `${base}/brave/res/v1/web/search?q=test`,
      { 'X-Subscription-Token': 'key-rest://user1/brave/api-key' });
    await runTest('gitlab', 'GET', `${base}/gitlab/api/v4/projects`,
      { 'Private-Token': 'key-rest://user1/gitlab/token' });
    await runTest('bing', 'GET', `${base}/bing/v7.0/search?q=test`,
      { 'Ocp-Apim-Subscription-Key': 'key-rest://user1/bing/api-key' });

    // --- Prefix/raw token services (2) ---
    await runTest('discord', 'POST', `${base}/discord/api/v10/channels/CH/messages`,
      { 'Authorization': 'Bot key-rest://user1/discord/bot-token', 'Content-Type': 'application/json' },
      '{"content":"test"}');
    await runTest('linear', 'POST', `${base}/linear/graphql`,
      { 'Authorization': 'key-rest://user1/linear/api-key', 'Content-Type': 'application/json' },
      '{"query":"{ issues { nodes { id title } } }"}');

    // --- Query parameter services (3) ---
    await runTest('gemini', 'POST',
      `${base}/gemini/v1beta/models/gemini-2.0-flash:generateContent?key=key-rest://user1/gemini/api-key`,
      { 'Content-Type': 'application/json' },
      '{"contents":[{"parts":[{"text":"test"}]}]}');
    await runTest('google-search', 'GET',
      `${base}/google-search/customsearch/v1?key=key-rest://user1/google-search/api-key`);
    await runTest('trello', 'GET',
      `${base}/trello/1/members/me/boards?key=key-rest://user1/trello/api-key&token=key-rest://user1/trello/token`);

    // --- Body field service (1) ---
    await runTest('tavily', 'POST', `${base}/tavily/search`,
      { 'Content-Type': 'application/json' },
      '{"api_key":"key-rest://user1/tavily/api-key","query":"test","search_depth":"basic"}');

    // --- Path embedding service (1) ---
    await runTest('telegram', 'POST',
      `${base}/telegram/bot{{key-rest://user1/telegram/bot-token}}/sendMessage`,
      { 'Content-Type': 'application/json' },
      '{"chat_id":123456789,"text":"test"}');

    // --- Basic auth service (1) ---
    await runTest('atlassian', 'GET',
      `${base}/atlassian/2.0/repositories/ws/repo/pullrequests`,
      { 'Authorization': 'Basic {{ base64(key-rest://user1/atlassian/email, ":", key-rest://user1/atlassian/token) }}' });

    // --- Response masking test ---
    console.log('');
    console.log('=== Testing response masking ===');
    console.log('');
    {
      total++;
      const echoKeyValue = creds['openai/api-key'];
      // Register a key for the echo endpoint
      const echoInput = `${passphrase}\n${echoKeyValue}\n`;
      execFileSync(keyRest, ['add', 'user1/echo/key', base + '/echo/'], {
        input: echoInput, env, stdio: 'pipe',
      });

      try {
        const resp = await fetch(`${base}/echo/test`, {
          method: 'GET',
          headers: { 'Authorization': 'Bearer key-rest://user1/echo/key' },
        });
        const text = await resp.text();
        if (text.includes(echoKeyValue)) {
          console.log('  FAIL  response-masking (credential leaked in response body)');
          failed++;
        } else if (!text.includes('key-rest://')) {
          console.log('  FAIL  response-masking (credential not reverse-substituted)');
          failed++;
        } else {
          console.log('  PASS  response-masking');
          passed++;
        }
      } catch (e) {
        console.log(`  FAIL  response-masking`);
        console.log(`        ${e.message}`);
        failed++;
      }
    }

    // --- Summary ---
    console.log('');
    console.log(`=== Results: ${passed}/${total} passed, ${failed} failed ===`);

    if (failed > 0) process.exit(1);

  } finally {
    if (daemonStarted) {
      try { execFileSync(keyRest, ['stop'], { env, stdio: 'pipe' }); } catch {}
    }
    if (testServerProc) {
      testServerProc.kill();
    }
    rmSync(work, { recursive: true, force: true });
  }
}

main().catch((e) => { console.error(e); process.exit(1); });
